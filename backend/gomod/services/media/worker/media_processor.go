package mediaworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	"goshop/contracts/events"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	"goshop/shared/pkg/storage"
)

// jpegQuality trades ~30-50% size for imperceptible quality loss on photos.
const jpegQuality = 82

// MediaProcessor resizes and re-encodes uploaded images so originals never
// reach clients at full resolution.
type MediaProcessor struct {
	storage storage.StorageService
	logger  *zerolog.Logger
}

func NewMediaProcessor(storageService storage.StorageService, log *zerolog.Logger) *MediaProcessor {
	return &MediaProcessor{storage: storageService, logger: log}
}

// Handle decodes a JPEG/PNG object, downscales it to fit 1920px on the long
// edge and re-encodes. Transparent PNGs stay PNG; anything else becomes JPEG.
// Videos and WebP are out of scope for this processor.
func (p *MediaProcessor) Handle(headers messaging.Header, body []byte) error {
	var evt events.MediaUploadedEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal media event: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(evt.FileKey))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return nil
	}

	ctx := context.Background()
	if traceID, ok := headers["trace_id"]; ok {
		ctx = logger.WithTraceID(ctx, traceID)
	}

	maxDim := 1920
	if strings.HasPrefix(evt.FileKey, "chat/") {
		maxDim = 2560 // chat attachments include screenshots; keep text readable
	}

	result, err := p.process(ctx, evt.FileKey, maxDim)
	if err != nil {
		return err
	}
	if !result.changed {
		p.logger.Debug().Str("file_key", evt.FileKey).Msg("Media within limits, left as is")
		return nil
	}

	// Overwrite the same key: URLs already handed to clients stay valid.
	folder, filename := splitKey(evt.FileKey)
	if _, _, err := p.storage.Upload(ctx, folder, filename, bytes.NewReader(result.bytes), int64(len(result.bytes)), result.contentType); err != nil {
		return fmt.Errorf("re-upload processed image: %w", err)
	}

	p.logger.Info().
		Str("file_key", evt.FileKey).
		Int64("before_bytes", evt.Size).
		Int("after_bytes", len(result.bytes)).
		Msg("Media re-encoded")
	return nil
}

type processResult struct {
	bytes       []byte
	contentType string
	changed     bool
}

// process returns re-encoded bytes; changed=false means keep the original.
// Corrupt payloads are logged and skipped rather than treated as failures.
func (p *MediaProcessor) process(ctx context.Context, fileKey string, maxDim int) (processResult, error) {
	reader, err := p.storage.Open(ctx, fileKey)
	if err != nil {
		return processResult{}, fmt.Errorf("open object: %w", err)
	}
	defer func() { _ = reader.Close() }()

	src, format, err := image.Decode(reader)
	if err != nil {
		p.logger.Warn().Err(err).Str("file_key", fileKey).Msg("Cannot decode image, skipping processing")
		return processResult{}, nil
	}

	bounds := src.Bounds()
	if bounds.Dx() <= maxDim && bounds.Dy() <= maxDim && format == "jpeg" {
		return processResult{}, nil // already small and already compressed
	}

	dst := downscale(src, maxDim)

	var buf bytes.Buffer
	if format == "png" && hasTransparency(src) {
		if err := png.Encode(&buf, dst); err != nil {
			return processResult{}, fmt.Errorf("encode png: %w", err)
		}
		return processResult{bytes: buf.Bytes(), contentType: "image/png", changed: true}, nil
	}

	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return processResult{}, fmt.Errorf("encode jpeg: %w", err)
	}
	return processResult{bytes: buf.Bytes(), contentType: "image/jpeg", changed: true}, nil
}

// downscale box-filters the image to fit maxDim on the long edge, in pixels.
func downscale(src image.Image, maxDim int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}

	scaleW := float64(w) / float64(maxDim)
	scaleH := float64(h) / float64(maxDim)
	scale := scaleW
	if scaleH > scale {
		scale = scaleH
	}
	nw, nh := int(float64(w)/scale), int(float64(h)/scale)

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy0 := int(float64(y) * scale)
		sy1 := int(float64(y+1) * scale)
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		if sy1 > h {
			sy1 = h
		}
		for x := 0; x < nw; x++ {
			sx0 := int(float64(x) * scale)
			sx1 := int(float64(x+1) * scale)
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			if sx1 > w {
				sx1 = w
			}

			var rSum, gSum, bSum, aSum, count uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					cr, cg, cb, ca := src.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
					rSum += uint64(cr)
					gSum += uint64(cg)
					bSum += uint64(cb)
					aSum += uint64(ca)
					count++
				}
			}
			dst.SetRGBA64(x, y, rgbaAvg(rSum, gSum, bSum, aSum, count))
		}
	}
	return dst
}

func rgbaAvg(rSum, gSum, bSum, aSum, count uint64) color.RGBA64 {
	if count == 0 {
		return color.RGBA64{}
	}
	return color.RGBA64{
		R: uint16(rSum / count),
		G: uint16(gSum / count),
		B: uint16(bSum / count),
		A: uint16(aSum / count),
	}
}

// hasTransparency reports whether any pixel is non-opaque. Sampling every ~8th
// pixel keeps it cheap; a missed semi-transparent row only affects PNG choice.
func hasTransparency(src image.Image) bool {
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
			if _, _, _, a := src.At(x, y).RGBA(); a>>8 < 255 {
				return true
			}
		}
	}
	return false
}

// splitKey splits "folder/name.ext" into its two Upload() arguments.
func splitKey(fileKey string) (folder, filename string) {
	folder, filename, _ = strings.Cut(strings.Trim(strings.TrimSpace(fileKey), "/"), "/")
	if folder == "" || filename == "" {
		return "common", fileKey
	}
	return folder, filename
}

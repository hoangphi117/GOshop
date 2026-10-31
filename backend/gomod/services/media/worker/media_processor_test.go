package mediaworker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	"goshop/contracts/events"
	"goshop/shared/pkg/messaging"
)

func nopLogger() *zerolog.Logger {
	l := zerolog.Nop()
	return &l
}

func mustEvent(t *testing.T, evt events.MediaUploadedEvent) []byte {
	t.Helper()
	data, err := json.Marshal(evt)
	require.NoError(t, err)
	return data
}

type fakeStorage struct {
	mu       sync.Mutex
	objects  map[string][]byte
	uploader func(folder, filename string, data []byte)
}

func (f *fakeStorage) Upload(_ context.Context, folder, filename string, reader io.Reader, _ int64, _ string) (string, string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", "", err
	}
	f.mu.Lock()
	f.objects[folder+"/"+filename] = data
	upload := f.uploader
	f.mu.Unlock()
	if upload != nil {
		upload(folder, filename, data)
	}
	return "https://cdn.example.com/" + folder + "/" + filename, folder + "/" + filename, nil
}

func (f *fakeStorage) Delete(context.Context, string) error { return nil }

func (f *fakeStorage) GeneratePresignedUploadURL(context.Context, string, string, string, time.Duration) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakeStorage) IsHealthy(context.Context) bool { return true }

func (f *fakeStorage) Stat(_ context.Context, fileKey string) (int64, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[fileKey]
	if !ok {
		return 0, "", assert.AnError
	}
	return int64(len(data)), "application/octet-stream", nil
}

func (f *fakeStorage) PublicURL(fileKey string) string { return "https://cdn.example.com/" + fileKey }

func (f *fakeStorage) RangeReader(_ context.Context, fileKey string, n int64) (io.ReadCloser, error) {
	f.mu.Lock()
	data, ok := f.objects[fileKey]
	f.mu.Unlock()
	if !ok {
		return nil, assert.AnError
	}
	if n > int64(len(data)) {
		n = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[:n])), nil
}

func (f *fakeStorage) Open(_ context.Context, fileKey string) (io.ReadCloser, error) {
	f.mu.Lock()
	data, ok := f.objects[fileKey]
	f.mu.Unlock()
	if !ok {
		return nil, assert.AnError
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func mustJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x % 256), uint8(y % 256), uint8((x + y) % 256), 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	return buf.Bytes()
}

func TestMediaProcessor_ResizesAndReencodesLargeJPEG(t *testing.T) {
	original := mustJPEG(t, 3200, 2000)
	store := &fakeStorage{objects: map[string][]byte{"products/big.jpg": original}}
	processor := NewMediaProcessor(store, nopLogger())

	err := processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{
		FileKey: "products/big.jpg",
		Size:    int64(len(original)),
	}))
	require.NoError(t, err)

	processed := store.objects["products/big.jpg"]
	require.NotNil(t, processed)
	assert.Less(t, len(processed), len(original), "re-encoded image should be smaller")

	img, format, err := image.Decode(bytes.NewReader(processed))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.LessOrEqual(t, img.Bounds().Dx(), 1920)
}

func TestMediaProcessor_SkipsSmallJPEG(t *testing.T) {
	original := mustJPEG(t, 400, 300)
	store := &fakeStorage{objects: map[string][]byte{"chat/tiny.jpg": original}}
	before := len(original)
	processor := NewMediaProcessor(store, nopLogger())

	require.NoError(t, processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{
		FileKey: "chat/tiny.jpg",
		Size:    int64(before),
	})))

	assert.Equal(t, before, len(store.objects["chat/tiny.jpg"]), "small JPEG must be untouched")
}

func TestMediaProcessor_SkipsVideoAndWebP(t *testing.T) {
	store := &fakeStorage{objects: map[string][]byte{}}
	processor := NewMediaProcessor(store, nopLogger())

	require.NoError(t, processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{FileKey: "chat/clip.mp4"})))
	require.NoError(t, processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{FileKey: "chat/pic.webp"})))
	assert.Empty(t, store.objects)
}

func TestMediaProcessor_SkipsCorruptImage(t *testing.T) {
	store := &fakeStorage{objects: map[string][]byte{"reviews/broken.jpg": []byte("this is not an image")}}
	processor := NewMediaProcessor(store, nopLogger())

	require.NoError(t, processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{FileKey: "reviews/broken.jpg"})))
	assert.Equal(t, "this is not an image", string(store.objects["reviews/broken.jpg"]), "corrupt file must be left alone")
}

func TestMediaProcessor_PreservesTransparentPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2600, 1400))
	for y := 0; y < 1400; y++ {
		for x := 0; x < 2600; x++ {
			img.Set(x, y, color.NRGBA{10, 20, 30, 128})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	original := buf.Bytes()

	store := &fakeStorage{objects: map[string][]byte{"returns/shot.png": original}}
	processor := NewMediaProcessor(store, nopLogger())

	require.NoError(t, processor.Handle(messaging.Header{}, mustEvent(t, events.MediaUploadedEvent{FileKey: "returns/shot.png", Size: int64(len(original))})))

	processed := store.objects["returns/shot.png"]
	require.NotNil(t, processed)
	dst, format, err := image.Decode(bytes.NewReader(processed))
	require.NoError(t, err)
	assert.Equal(t, "png", format)
	_, _, _, alpha := dst.At(0, 0).RGBA()
	assert.Less(t, alpha, uint32(65535), "transparency must survive")
	assert.LessOrEqual(t, dst.Bounds().Dx(), 1920)
}

func TestMediaProcessor_SplitKey(t *testing.T) {
	folder, name := splitKey("chat/abc.jpg")
	assert.Equal(t, "chat", folder)
	assert.Equal(t, "abc.jpg", name)

	folder, name = splitKey("junk-without-slash.jpg")
	assert.Equal(t, "common", folder)
	assert.True(t, strings.HasSuffix(name, ".jpg"))
}

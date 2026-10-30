package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

type traceIDContextKey struct{}

var Log *zerolog.Logger

func init() {
	nop := zerolog.Nop()
	Log = &nop
}

type LoggerConfig struct {
	Level      string
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
	IsDev      string
}

func InitLogger(config LoggerConfig) {
	Log = NewLogger(config)
}

func NewLogger(config LoggerConfig) *zerolog.Logger {

	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var writer io.Writer

	if config.IsDev == "development" {
		writer = PrettyJSONWriter{Writer: os.Stdout}
	} else if err := os.MkdirAll(filepath.Dir(config.Filename), 0o755); err == nil {
		writer = &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize, // megabytes
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,   //days
			Compress:   config.Compress, // disabled by default
		}
	} else {
		// Directory creation failed (read-only FS etc.): stdout is still
		// collected by the container runtime / shipped to Loki by promtail.
		writer = os.Stdout
	}

	logger := zerolog.New(writer).Level(lvl).With().Timestamp().Logger()

	return &logger
}

type PrettyJSONWriter struct {
	Writer io.Writer
}

func (w PrettyJSONWriter) Write(p []byte) (n int, err error) {
	var prettyJSON bytes.Buffer

	err = json.Indent(&prettyJSON, p, "", "  ")
	if err != nil {
		return w.Writer.Write(p)
	}

	return w.Writer.Write(prettyJSON.Bytes())
}

func GetTraceId(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return traceID
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return traceID
	}
	return ""
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

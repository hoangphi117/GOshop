package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type traceIDContextKey struct{}

var Log *zerolog.Logger

func init() {
	nop := zerolog.Nop()
	Log = &nop
}

// LoggerConfig keeps the pre-stdout field names so callers do not change.
// Filename, MaxSize, MaxBackups, MaxAge and Compress are accepted and ignored:
// every process writes to stdout.
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

// NewLogger writes to stdout.
//
// A containerised process has nobody reading a file it writes: promtail tails
// the Docker log file and ships it to Loki. Rotating into the container
// filesystem loses the lines that matter and keeps a copy that dies with the
// container.
//
// ponytail: no size-based rotation. Add it when a bare-metal run needs one —
// systemd/journald or a log shipper already covers that case.
func NewLogger(config LoggerConfig) *zerolog.Logger {

	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var writer io.Writer = os.Stdout
	if config.IsDev == "development" {
		writer = PrettyJSONWriter{Writer: os.Stdout}
	}

	logger := zerolog.New(writer).Level(lvl).With().Timestamp().Logger()

	return &logger
}

// PrettyJSONWriter indents one JSON line for local development. A line it
// cannot parse is written unchanged, which is what zerolog expects.
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

package eventledger

import "github.com/rs/zerolog"

type zerologAdapter struct {
	logger *zerolog.Logger
}

// NewZerologLogger adapts a zerolog logger to the Logger interface.
func NewZerologLogger(logger *zerolog.Logger) Logger {
	return &zerologAdapter{logger: logger}
}

func (a *zerologAdapter) Info(msg string, fields map[string]any) {
	a.log(a.logger.Info(), msg, fields)
}

func (a *zerologAdapter) Error(err error, msg string, fields map[string]any) {
	a.log(a.logger.Error().Err(err), msg, fields)
}

func (a *zerologAdapter) log(event *zerolog.Event, msg string, fields map[string]any) {
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg(msg)
}

package aibot

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

type DefaultLogger struct {
	Debug bool
	Out   io.Writer
}

func (l *DefaultLogger) Debugf(format string, args ...any) {
	if l.Debug {
		l.output("DEBUG", format, args...)
	}
}

func (l *DefaultLogger) Infof(format string, args ...any) {
	l.output("INFO", format, args...)
}

func (l *DefaultLogger) Warnf(format string, args ...any) {
	l.output("WARN", format, args...)
}

func (l *DefaultLogger) Errorf(format string, args ...any) {
	l.output("ERROR", format, args...)
}

func (l *DefaultLogger) output(level, format string, args ...any) {
	w := l.Out
	if w == nil {
		w = os.Stderr
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(w, "%s [%s] aibot: %s\n", ts, level, fmt.Sprintf(format, args...))
}

type NopLogger struct{}

func (NopLogger) Debugf(string, ...any) {}

func (NopLogger) Infof(string, ...any) {}

func (NopLogger) Warnf(string, ...any) {}

func (NopLogger) Errorf(string, ...any) {}

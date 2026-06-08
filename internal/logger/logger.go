package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/drunkleen/l-ui/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	maxLogBufferSize = 10240
	logFileName      = "3lui.log"
	timeFormat       = "2006/01/02 15:04:05"

	maxLogFileMB    = 10
	maxLogBackups   = 5
	maxLogAgeDays   = 7
	compressRotated = true
)

var (
	slogger    *slog.Logger
	fileRotate *lumberjack.Logger

	logBuffer []struct {
		time  string
		level slog.Level
		log   string
	}
)

type multiWriter struct {
	writers []io.Writer
}

func (m *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
		_, err = w.Write(p)
		if err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func InitLogger(levelStr string) {
	logFormat := os.Getenv("LUI_LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text"
	}

	fileRotate = initFileBackend()

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: levelFromString(levelStr)}

	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	if fileRotate != nil {
		writers := []io.Writer{fileRotate}
		if logFormat == "json" {
			handler = slog.NewJSONHandler(&multiWriter{writers}, opts)
		} else {
			handler = slog.NewTextHandler(&multiWriter{writers}, opts)
		}
	}

	slogger = slog.New(handler)
}

func initFileBackend() *lumberjack.Logger {
	logDir := config.GetLogFolder()
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log folder %s: %v\n", logDir, err)
		return nil
	}

	logPath := filepath.Join(logDir, logFileName)
	return &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    maxLogFileMB,
		MaxBackups: maxLogBackups,
		MaxAge:     maxLogAgeDays,
		LocalTime:  true,
		Compress:   compressRotated,
	}
}

func levelFromString(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "notice":
		return slog.LevelInfo
	case "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func CloseLogger() {
	if fileRotate != nil {
		_ = fileRotate.Close()
		fileRotate = nil
	}
}

func Debug(args ...any) {
	slogger.Debug(fmt.Sprint(args...))
	addToBuffer(slog.LevelDebug, fmt.Sprint(args...))
}

func Debugf(format string, args ...any) {
	slogger.Debug(fmt.Sprintf(format, args...))
	addToBuffer(slog.LevelDebug, fmt.Sprintf(format, args...))
}

func Info(args ...any) {
	slogger.Info(fmt.Sprint(args...))
	addToBuffer(slog.LevelInfo, fmt.Sprint(args...))
}

func Infof(format string, args ...any) {
	slogger.Info(fmt.Sprintf(format, args...))
	addToBuffer(slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Notice(args ...any) {
	slogger.Info(fmt.Sprint(args...))
	addToBuffer(slog.LevelInfo, fmt.Sprint(args...))
}

func Noticef(format string, args ...any) {
	slogger.Info(fmt.Sprintf(format, args...))
	addToBuffer(slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Warning(args ...any) {
	slogger.Warn(fmt.Sprint(args...))
	addToBuffer(slog.LevelWarn, fmt.Sprint(args...))
}

func Warningf(format string, args ...any) {
	slogger.Warn(fmt.Sprintf(format, args...))
	addToBuffer(slog.LevelWarn, fmt.Sprintf(format, args...))
}

func Error(args ...any) {
	slogger.Error(fmt.Sprint(args...))
	addToBuffer(slog.LevelError, fmt.Sprint(args...))
}

func Errorf(format string, args ...any) {
	slogger.Error(fmt.Sprintf(format, args...))
	addToBuffer(slog.LevelError, fmt.Sprintf(format, args...))
}

func addToBuffer(level slog.Level, newLog string) {
	t := time.Now()
	if len(logBuffer) >= maxLogBufferSize {
		logBuffer = logBuffer[1:]
	}

	logBuffer = append(logBuffer, struct {
		time  string
		level slog.Level
		log   string
	}{
		time:  t.Format(timeFormat),
		level: level,
		log:   newLog,
	})
}

func GetLogs(c int, level string) []string {
	var output []string
	logLevel := levelFromString(level)

	for i := len(logBuffer) - 1; i >= 0 && len(output) <= c; i-- {
		if logBuffer[i].level >= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", logBuffer[i].time, logBuffer[i].level, logBuffer[i].log))
		}
	}
	return output
}

func GetSlogger() *slog.Logger {
	return slog.Default()
}

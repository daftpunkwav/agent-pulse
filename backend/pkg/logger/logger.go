// Package logger 提供统一的日志接口。
//
// 基于 zerolog 实现，支持两种输出格式：
//   - json：生产环境友好，便于日志聚合系统解析
//   - console：开发环境友好，带颜色与时间戳
//
// 所有模块应使用本包的接口，避免直接依赖 zerolog，便于后续替换实现。
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Level 日志级别。
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelFatal Level = "fatal"
)

// Format 日志输出格式。
type Format string

const (
	FormatJSON    Format = "json"
	FormatConsole Format = "console"
)

// Logger 是 AgentPulse 的日志接口。
//
// 设计为最小公共子集，所有模块只依赖此接口。
// 替换底层实现（如换 zap/slog）只需修改本包。
type Logger interface {
	// 基础字段方法（与 zerolog 对齐）
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)

	// 上下文方法
	WithField(key string, value any) Logger
	WithFields(fields map[string]any) Logger
	WithError(err error) Logger

	// 输出控制
	Sync() error
}

// zerologLogger 是 Logger 接口的 zerolog 实现。
type zerologLogger struct {
	logger zerolog.Logger
}

// New 创建新的日志实例。
//
// 参数：
//   - level: debug/info/warn/error/fatal
//   - format: json/console
func New(level, format string) (Logger, error) {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	zerolog.SetGlobalLevel(parsedLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var writer io.Writer
	if strings.ToLower(format) == "console" {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.MessageFieldName,
			},
		}
	} else {
		writer = os.Stdout
	}

	base := zerolog.New(writer).
		With().
		Timestamp().
		Str("service", "agentpulse").
		Logger()

	return &zerologLogger{logger: base}, nil
}

// NewWithWriter 用于测试场景，允许注入自定义 writer。
func NewWithWriter(level Level, format Format, w io.Writer) Logger {
	zerolog.SetGlobalLevel(zerolog.Level(parseLevelUnsafe(level)))

	var base zerolog.Logger
	if format == FormatConsole {
		writer := zerolog.ConsoleWriter{Out: w, NoColor: true}
		base = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		base = zerolog.New(w).With().Timestamp().Logger()
	}

	return &zerologLogger{logger: base}
}

// NewNop 创建一个空日志（用于测试，禁用所有输出）。
func NewNop() Logger {
	return &zerologLogger{logger: zerolog.Nop()}
}

// ---------------------------------------------------------------------------
// 接口实现
// ---------------------------------------------------------------------------

func (l *zerologLogger) Debugf(format string, args ...any) {
	l.logger.Debug().Msg(fmt.Sprintf(format, args...))
}

func (l *zerologLogger) Infof(format string, args ...any) {
	l.logger.Info().Msg(fmt.Sprintf(format, args...))
}

func (l *zerologLogger) Warnf(format string, args ...any) {
	l.logger.Warn().Msg(fmt.Sprintf(format, args...))
}

func (l *zerologLogger) Errorf(format string, args ...any) {
	l.logger.Error().Msg(fmt.Sprintf(format, args...))
}

func (l *zerologLogger) Fatalf(format string, args ...any) {
	l.logger.Fatal().Msg(fmt.Sprintf(format, args...))
}

func (l *zerologLogger) WithField(key string, value any) Logger {
	return &zerologLogger{
		logger: l.logger.With().Interface(key, value).Logger(),
	}
}

func (l *zerologLogger) WithFields(fields map[string]any) Logger {
	ctx := l.logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &zerologLogger{logger: ctx.Logger()}
}

func (l *zerologLogger) WithError(err error) Logger {
	return &zerologLogger{
		logger: l.logger.With().Err(err).Logger(),
	}
}

func (l *zerologLogger) Sync() error {
	// zerolog 无缓冲，Sync 为 no-op
	return nil
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

func parseLevel(level string) (zerolog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "fatal":
		return zerolog.FatalLevel, nil
	case "":
		return zerolog.InfoLevel, nil
	default:
		return zerolog.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

func parseLevelUnsafe(level Level) zerolog.Level {
	switch strings.ToLower(string(level)) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}
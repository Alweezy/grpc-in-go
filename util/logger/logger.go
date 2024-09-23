package logger

import (
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

type LogLevelType string

func (l *LogLevelType) Get() logrus.Level {
	switch *l {
	case "debug":
		return logrus.DebugLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

type LogOutputType string

func (o *LogOutputType) IsConsole() bool {
	return *o == "console"
}

func (o *LogOutputType) IsFile() bool {
	return *o == "file"
}

type LogConfig struct {
	LogLevel        LogLevelType  `mapstructure:"level"`
	LogOutputType   LogOutputType `mapstructure:"output_type"`
	LogTargetFolder string        `mapstructure:"path"`
	LogFileName     string        `mapstructure:"filename"`
}

// LoadLogger configures the formatter and output methods
func LoadLogger(config *LogConfig, formatter logrus.Formatter) error {
	logrus.SetFormatter(formatter)
	logrus.SetLevel(config.LogLevel.Get())

	err := setLogOutput(config.LogOutputType, config.LogTargetFolder, config.LogFileName)
	if err != nil {
		return err
	}

	return nil
}

// setLogOutput sets the log output medium based on the config
func setLogOutput(logOutput LogOutputType, targetFolder string, fileName string) error {
	if logOutput.IsConsole() {
		logrus.SetOutput(os.Stdout)
		return nil
	}

	if logOutput.IsFile() {
		if err := os.MkdirAll(targetFolder, os.ModePerm); err != nil {
			return err
		}

		logPath := targetFolder + "/%Y-%m-%d-" + fileName + ".log"
		logPathLink := targetFolder + "/" + fileName + ".log"

		writer, err := rotatelogs.New(
			logPath,
			rotatelogs.WithLinkName(logPathLink), // Create a symlink to the latest log file
			rotatelogs.WithRotationTime(24*time.Hour), // Rotate every 24 hours
			rotatelogs.WithMaxAge(7*24*time.Hour),     // Keep logs for 7 days
		)

		if err != nil {
			return err
		}

		logrus.SetOutput(writer)
	}

	return nil
}

// LogContext is a custom type for structured log context
type LogContext map[string]interface{}

// AddContext adds context to the LogContext
func (c *LogContext) AddContext(key string, value interface{}) {
	(*c)[key] = value
}

// LogDebug Logging functions for different levels
func LogDebug(msg string, ctx *LogContext) {
	logrus.WithFields(logrus.Fields(*ctx)).Debug(msg)
}

func LogInfo(msg string, ctx *LogContext) {
	logrus.WithFields(logrus.Fields(*ctx)).Info(msg)
}

func LogWarn(msg string, ctx *LogContext) {
	logrus.WithFields(logrus.Fields(*ctx)).Warn(msg)
}

func LogError(msg string, cause error, ctx *LogContext) {
	logrus.WithError(cause).WithFields(logrus.Fields(*ctx)).Error(msg)
}

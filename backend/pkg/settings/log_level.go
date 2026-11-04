package settings

import (
	"fmt"

	"github.com/gtsteffaniak/go-logger/logger"
)

// LogLevels maps the level the app settings offer to the levels the logger
// writes.
var LogLevels = map[string]string{
	"error": "error",
	"info":  "info|warning|error",
	"debug": "debug|info|warning|error",
}

// SetLogLevel rebuilds the loggers of every configured output with the
// given level, one of the keys of LogLevels.
func SetLogLevel(level string) error {
	levels, ok := LogLevels[level]
	if !ok {
		return fmt.Errorf("unknown log level %q", level)
	}
	if isGoTest() {
		return nil
	}
	outputs := Config.Server.Logging
	if len(outputs) == 0 {
		outputs = []LogConfig{{Output: "stdout"}}
	}
	logger.DisableCompatibilityMode()
	for _, cfg := range outputs {
		pattern := cfg.ApiFilter
		if pattern == "" {
			pattern = "^/health|^/favicon.ico|^/static|^/public/static"
		}
		err := logger.EnableCompatibilityMode(logger.JsonConfig{
			Levels:         levels,
			ApiLevels:      cfg.ApiLevels,
			Output:         cfg.Output,
			Utc:            cfg.Utc,
			NoColors:       cfg.NoColors,
			Json:           cfg.Json,
			ApiPathExclude: pattern,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

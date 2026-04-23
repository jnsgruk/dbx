package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var (
	version = "dev"
	commit  = "dev"
	date    string
)

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func setupLogger(level string) {
	logLevel := new(slog.LevelVar)
	logLevel.Set(logLevels[strings.ToLower(level)])
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05"))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(h))
}

func buildVersion(version, commit, date string) string {
	result := version
	if commit != "" {
		result = fmt.Sprintf("%s\ncommit: %s", result, commit)
	}
	if date != "" {
		result = fmt.Sprintf("%s\nbuilt at: %s", result, date)
	}
	return result
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

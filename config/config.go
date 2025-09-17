package config

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"runtime"
	"path/filepath"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("XUI_DEBUG") == "true"
}

func GetDBPath() string {
	// 使用本地 data 目录在 Debug 或 Windows 环境下，便于开发与避免权限问题
	if IsDebug() || runtime.GOOS == "windows" {
		return filepath.Join(".", "data", fmt.Sprintf("%s.db", GetName()))
	}
	return fmt.Sprintf("/etc/%s/%s.db", GetName(), GetName())
}

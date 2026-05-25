package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/colors"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
)

var (
	logger    *slog.Logger
	file      *os.File
	logPath   string
	logLevel  Level
	startedAt time.Time
)

// Level 表示日志级别
type Level = slog.Level

const (
	// DEBUG 表示调试级别
	DEBUG = slog.LevelDebug
	// INFO 表示信息级别
	INFO = slog.LevelInfo
	// WARN 表示警告级别
	WARN = slog.LevelWarn
	// ERROR 表示错误级别
	ERROR = slog.LevelError
)

// ParseLevel 从字符串解析日志级别
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "WARN", "warn", "WARNING", "warning":
		return WARN
	case "ERROR", "error":
		return ERROR
	default:
		return INFO
	}
}

// Init 初始化日志记录器
// 日志目录：日志文件夹路径
// 命令名称：当前执行的命令
// 级别：日志级别
func Init(logDir string, commandName string, level Level) error {
	return InitWithRetention(logDir, commandName, level, 0)
}

// InitWithRetention 初始化日志记录器，并在配置保留数量时清理旧日志文件
func InitWithRetention(logDir string, commandName string, level Level, maxLogFiles int) error {
	if file != nil {
		_ = file.Close()
		file = nil
	}

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 生成日志文件名：命令名加时间戳
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s-%s.log", commandName, timestamp)
	path := filepath.Join(logDir, logFileName)

	// 打开日志文件
	var err error
	file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logPath = filepath.Clean(path)
	logLevel = level
	startedAt = time.Now()

	// 创建只写入文件的结构化处理器。调用点会记录模块和行号，
	// 排查长耗时或异常路径时能直接定位到模块和行号
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}
	fileHandler := slog.NewJSONHandler(file, opts)

	// 创建只写入文件的日志记录器
	logger = slog.New(fileHandler)

	Diagnostic(i18n.Get("LoggerRuntimeInitialized"),
		"command", commandName,
		"log_path", logPath,
		"level", level.String(),
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"args", os.Args,
		"cwd", currentWorkingDir(),
	)

	return cleanupOldLogFiles(logDir, maxLogFiles)
}

// Close 关闭日志文件
func Close() error {
	if file != nil {
		Diagnostic(i18n.Get("LoggerRuntimeClosing"),
			"log_path", logPath,
			"uptime", time.Since(startedAt),
		)
		err := file.Close()
		file = nil
		logger = nil
		return err
	}
	return nil
}

// Debug 记录调试信息，只写文件，不输出控制台
func Debug(msg string, args ...any) {
	// 只写入日志文件
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

// Diagnostic 记录关键诊断信息，只写日志文件，不输出到终端
// 用于阶段边界、耗时、提示词和代理尺寸等排障信息，避免终端被调试日志刷屏
func Diagnostic(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

// Info 记录一般信息，并同步写入控制台和文件
func Info(msg string, args ...any) {
	// 控制台蓝色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLine(colorize(msg, colors.White))

	// 写入日志文件（详细信息）
	if logger != nil {
		logger.Info(msg, args...)
	}
}

// Warn 记录警告信息，并同步写入控制台和文件
func Warn(msg string, args ...any) {
	// 控制台黄色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLine(colorize(msg, colors.Yellow))

	// 写入日志文件（详细信息）
	if logger != nil {
		logger.Warn(msg, args...)
	}
}

// Error 记录错误信息，并同步写入控制台和文件
func Error(msg string, args ...any) {
	// 控制台红色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLine(colorize(msg, colors.Red))

	// 写入日志文件（详细信息）
	if logger != nil {
		logger.Error(msg, args...)
	}
}

// With 返回带有上下文字段的日志记录器
func With(args ...any) *slog.Logger {
	if logger != nil {
		return logger.With(args...)
	}
	return nil
}

// CurrentLogPath 返回当前日志路径
func CurrentLogPath() string {
	return logPath
}

// CurrentLevel 返回当前日志级别
func CurrentLevel() Level {
	return logLevel
}

// 添加颜色
func colorize(text string, color string) string {
	return color + text + colors.Reset
}

func currentWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func cleanupOldLogFiles(logDir string, maxLogFiles int) error {
	if maxLogFiles <= 0 {
		return nil
	}

	matches, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		return err
	}
	if len(matches) <= maxLogFiles {
		return nil
	}

	type logFile struct {
		path    string
		modTime time.Time
	}
	files := make([]logFile, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, logFile{path: match, modTime: info.ModTime()})
	}
	if len(files) <= maxLogFiles {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	for _, old := range files[maxLogFiles:] {
		if filepath.Clean(old.path) == logPath {
			continue
		}
		if err := os.Remove(old.path); err != nil {
			return err
		}
	}
	return nil
}

package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/colors"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
)

var (
	mu         sync.Mutex
	logger     *slog.Logger
	file       *os.File
	logPath    string
	logLevel   Level
	startedAt  time.Time
	scopedLogs = map[uint64]*scopedLog{}
)

type scopedLog struct {
	logger    *slog.Logger
	file      *os.File
	logPath   string
	startedAt time.Time
}

type scopedLogContextKey struct{}

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
	mu.Lock()
	defer mu.Unlock()

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

	writeDiagnosticLocked(i18n.Get("LoggerRuntimeInitialized"),
		"command", commandName,
		"log_path", logPath,
		"level", level.String(),
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"args", os.Args,
		"cwd", currentWorkingDir(),
	)

	return cleanupOldLogFiles(logDir, maxLogFiles, logPath)
}

// Close 关闭日志文件
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if file != nil {
		writeDiagnosticLocked(i18n.Get("LoggerRuntimeClosing"),
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
	if logger := currentLogger(); logger != nil {
		logger.Debug(msg, args...)
	}
}

// Diagnostic 记录关键诊断信息，只写日志文件，不输出到终端
// 用于阶段边界、耗时、提示词和代理尺寸等排障信息，避免终端被调试日志刷屏
func Diagnostic(msg string, args ...any) {
	if logger := currentLogger(); logger != nil {
		logger.Info(msg, args...)
	}
}

// Info 记录一般信息，并同步写入控制台和文件
func Info(msg string, args ...any) {
	// 控制台蓝色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLine(colorize(msg, colors.White))

	// 写入日志文件（详细信息）
	if logger := currentLogger(); logger != nil {
		logger.Info(msg, args...)
	}
}

// InfoAfterProgress 记录一般信息；控制台输出会等当前进度步骤结束后再打印
func InfoAfterProgress(msg string, args ...any) {
	progress.PrintConsoleLineAfterProgress(colorize(msg, colors.White))

	if logger := currentLogger(); logger != nil {
		logger.Info(msg, args...)
	}
}

// Warn 记录警告信息，并同步写入控制台和文件
func Warn(msg string, args ...any) {
	// 控制台黄色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLine(colorize(msg, colors.Yellow))

	// 写入日志文件（详细信息）
	if logger := currentLogger(); logger != nil {
		logger.Warn(msg, args...)
	}
}

// Error 记录错误信息，并同步写入控制台和文件
func Error(msg string, args ...any) {
	// 控制台红色输出（用户提示） - 只输出消息本身
	progress.PrintConsoleLineNow(colorize(msg, colors.Red))

	// 写入日志文件（详细信息）
	if logger := currentLogger(); logger != nil {
		logger.Error(msg, args...)
	}
}

// With 返回带有上下文字段的日志记录器
func With(args ...any) *slog.Logger {
	if logger := currentLogger(); logger != nil {
		return logger.With(args...)
	}
	return nil
}

// CurrentLogPath 返回当前日志路径
func CurrentLogPath() string {
	mu.Lock()
	defer mu.Unlock()

	return logPath
}

// CurrentLevel 返回当前日志级别
func CurrentLevel() Level {
	mu.Lock()
	defer mu.Unlock()

	return logLevel
}

// WithScopedLog 运行 fn，并把当前 goroutine 的文件日志路由到独立日志文件。
// 控制台输出保持不变。
func WithScopedLog(ctx context.Context, logDir string, commandName string, level Level, maxLogFiles int, fn func(context.Context, string) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	gid := currentGoroutineID()
	if gid == 0 {
		return fn(ctx, "")
	}

	scope, err := openScopedLog(logDir, commandName, level)
	if err != nil {
		return err
	}
	if err := registerScopedLog(gid, scope); err != nil {
		_ = scope.file.Close()
		return err
	}
	if err := cleanupOldLogFiles(logDir, maxLogFiles, scope.logPath); err != nil {
		unregisterScopedLog(gid, scope)
		_ = scope.file.Close()
		return err
	}

	scope.logger.Info(i18n.Get("LoggerRuntimeInitialized"),
		"command", commandName,
		"log_path", scope.logPath,
		"level", level.String(),
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
		"args", os.Args,
		"cwd", currentWorkingDir(),
	)

	scopedCtx := context.WithValue(ctx, scopedLogContextKey{}, scope)
	err = fn(scopedCtx, scope.logPath)
	if err != nil {
		scope.logger.Error(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", commandName,
			"error", err,
		)
	}
	scope.logger.Info(i18n.Get("LoggerRuntimeClosing"),
		"log_path", scope.logPath,
		"uptime", time.Since(scope.startedAt),
	)
	unregisterScopedLog(gid, scope)
	if closeErr := scope.file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// BindScope 将 ctx 中的日志作用域绑定到当前 goroutine。
// 并发 worker 在启动时调用，并在退出时执行返回的释放函数。
func BindScope(ctx context.Context) func() {
	if ctx == nil {
		return func() {}
	}
	scope, _ := ctx.Value(scopedLogContextKey{}).(*scopedLog)
	if scope == nil {
		return func() {}
	}
	gid := currentGoroutineID()
	if gid == 0 {
		return func() {}
	}

	mu.Lock()
	previous := scopedLogs[gid]
	scopedLogs[gid] = scope
	mu.Unlock()
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if scopedLogs[gid] != scope {
			return
		}
		if previous == nil {
			delete(scopedLogs, gid)
			return
		}
		scopedLogs[gid] = previous
	}
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

func currentLogger() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	if scoped := scopedLogs[currentGoroutineID()]; scoped != nil {
		return scoped.logger
	}
	return logger
}

func writeDiagnosticLocked(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

func openScopedLog(logDir string, commandName string, level Level) (*scopedLog, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s-%s.log", commandName, timestamp)
	path := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &scopedLog{
		logger: slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		})),
		file:      logFile,
		logPath:   filepath.Clean(path),
		startedAt: time.Now(),
	}, nil
}

func registerScopedLog(gid uint64, scope *scopedLog) error {
	mu.Lock()
	defer mu.Unlock()

	if scopedLogs[gid] != nil {
		return fmt.Errorf("logger scoped log already registered for goroutine %d", gid)
	}
	scopedLogs[gid] = scope
	return nil
}

func unregisterScopedLog(gid uint64, scope *scopedLog) {
	mu.Lock()
	defer mu.Unlock()

	if scopedLogs[gid] == scope {
		delete(scopedLogs, gid)
	}
}

func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 2 || fields[0] != "goroutine" {
		return 0
	}
	id, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func cleanupOldLogFiles(logDir string, maxLogFiles int, protectedPaths ...string) error {
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

	protected := make(map[string]struct{}, len(protectedPaths))
	for _, path := range protectedPaths {
		if path != "" {
			protected[filepath.Clean(path)] = struct{}{}
		}
	}

	for _, old := range files[maxLogFiles:] {
		if _, ok := protected[filepath.Clean(old.path)]; ok {
			continue
		}
		if err := os.Remove(old.path); err != nil {
			return err
		}
	}
	return nil
}

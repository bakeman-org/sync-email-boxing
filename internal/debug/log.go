package debug

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"sync-email-boxing/internal/config"
)

var (
	mu      sync.Mutex
	logger  *log.Logger
	logPath string
	file    *os.File
)

// DefaultPath 日志文件默认位置（与配置文件同目录）
func DefaultPath() string {
	return filepath.Join(filepath.Dir(config.Path()), "debug.log")
}

// Init 初始化日志。
// enabled=false（默认，日常使用）：不创建日志文件，所有输出直接丢弃；
// enabled=true（开发者模式，-v/--debug）：写入日志文件（含 DEBUG 细节）。
func Init(path string, enabled bool) error {
	mu.Lock()
	defer mu.Unlock()
	if !enabled {
		logPath = ""
		logger = nil
		file = nil
		return nil
	}
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	file = f
	logPath = path
	logger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("===== 启动 debug=on pid=%d =====", os.Getpid())
	return nil
}

// Close 关闭日志文件
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		logger.Printf("===== 退出 =====")
		file.Close()
		file = nil
		logger = nil
	}
}

// Path 当前日志文件路径；未开启日志时返回空串
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

func output(level, loc, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		return
	}
	logger.Printf("[%s] %s %s", level, loc, fmt.Sprintf(format, args...))
}

// callerInfo 模拟 C 的 __FILE__:__LINE__：返回调用处的 "文件名:行号:"。
// skip 相对 runtime.Caller：0=callerInfo 自身，1=Infof 等包装函数，2=用户调用处
func callerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "?:"
	}
	return fmt.Sprintf("%s:%d:", path.Base(file), line)
}

// Infof 关键流程日志（开启日志时写入，自动带调用处 文件:行号）
func Infof(format string, args ...any) { output("INFO", callerInfo(2), format, args...) }

// Errf 错误日志（开启日志时写入，自动带调用处 文件:行号）
func Errf(format string, args ...any) { output("ERROR", callerInfo(2), format, args...) }

// Debugf 调试细节（与 Infof 同级：日志开启即写入，自动带调用处 文件:行号）
func Debugf(format string, args ...any) { output("DEBUG", callerInfo(2), format, args...) }

// Since 用于耗时统计，返回毫秒数
func Since(t time.Time) string {
	return fmt.Sprintf("%.0fms", float64(time.Since(t).Microseconds())/1000)
}

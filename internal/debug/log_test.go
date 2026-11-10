package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCallerFileLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "d.log")
	if err := Init(p, true); err != nil {
		t.Fatal(err)
	}
	defer func() { mu.Lock(); file.Close(); file = nil; logger = nil; mu.Unlock() }()

	Infof("hello %s", "world")
	Errf("bad thing")

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "log_test.go:18") {
		t.Errorf("Infof 未带调用处行号:\n%s", s)
	}
	if !strings.Contains(s, "log_test.go:19") {
		t.Errorf("Errf 未带调用处行号:\n%s", s)
	}
	if !strings.Contains(s, "[INFO] log_test.go:18: hello world") {
		t.Errorf("格式不对:\n%s", s)
	}
}

func TestDisabledDiscards(t *testing.T) {
	if err := Init("", false); err != nil {
		t.Fatal(err)
	}
	if Path() != "" {
		t.Errorf("未开启日志时 Path 应为空: %q", Path())
	}
	// 不应 panic，且不产生任何文件
	Infof("should be discarded")
	Errf("also discarded")
}

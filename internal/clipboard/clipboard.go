package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Copy 把文本复制到系统剪贴板。
// 依次尝试 wl-copy(Wayland)、xclip/xsel(X11)，都不可用时回退到
// OSC52 终端转义序列（SSH/远程终端也能用）。
func Copy(text string) error {
	tries := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, args := range tries {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// OSC52 回退：直接写终端
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	if len(b64) > 100000 {
		return fmt.Errorf("内容过大（%d 字节），终端剪贴板不支持，请安装 xclip 或 wl-copy", len(b64))
	}
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("无可用剪贴板（缺少 xclip/wl-copy，且无法访问终端）: %w", err)
	}
	defer tty.Close()
	_, err = tty.WriteString("\x1b]52;c;" + b64 + "\x07")
	return err
}

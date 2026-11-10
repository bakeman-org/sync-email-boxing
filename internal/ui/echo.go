package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// echo area（emacs 风格回显区）：
// 任何按键动作的反馈都显示在 TUI 最底部的消息栏——
// 成功为绿色 ✓，失败为红色 ✗，超时后自动清空。

const (
	echoStatusTimeout = 5 * time.Second
	echoErrTimeout    = 8 * time.Second
)

type echoTickMsg struct{}

// echoClearCmd 定时触发回显区清空检查
func echoClearCmd() tea.Cmd {
	return tea.Tick(echoErrTimeout, func(time.Time) tea.Msg { return echoTickMsg{} })
}

// setStatus 记录成功反馈并安排超时清除
func (m *Model) setStatus(s string) tea.Cmd {
	m.status = s
	m.statusAt = time.Now()
	m.errMsg = ""
	m.errAt = time.Time{}
	return echoClearCmd()
}

// setError 记录失败反馈并安排超时清除
func (m *Model) setError(s string) tea.Cmd {
	m.errMsg = s
	m.errAt = time.Now()
	m.status = ""
	m.statusAt = time.Time{}
	return echoClearCmd()
}

// echoExpired 回显消息是否已超时
func (m Model) echoExpired() bool {
	if m.errMsg != "" && time.Since(m.errAt) >= echoErrTimeout {
		return true
	}
	if m.status != "" && time.Since(m.statusAt) >= echoStatusTimeout {
		return true
	}
	return false
}

// echoMode 当前视图名，显示在回显行最左侧
func echoMode(m Model) string {
	switch m.mode {
	case modeForm:
		return "添加账号"
	case modeAccounts:
		return "账号"
	case modeFolders:
		return "文件夹"
	case modeViewer:
		return "阅读"
	default:
		return "收件箱"
	}
}

// renderEcho 渲染底部回显区（占一行，空时显示模式名避免布局跳动）
func renderEcho(m Model) string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	prefix := helpStyle.Render("[" + echoMode(m) + "]")
	switch {
	case m.errMsg != "":
		return prefix + errStyle.Render(" ✗ "+truncateW(m.errMsg, w-14))
	case m.status != "":
		return prefix + statusStyle.Render(" ✓ "+truncateW(m.status, w-14))
	default:
		return prefix + dimStyle.Render(" （就绪）")
	}
}

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// keyHint 单个按键提示
type keyHint struct{ key, desc string }

// k 快捷构造 keyHint
func k(key, desc string) keyHint { return keyHint{key, desc} }

var keyStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#11111b")).
	Background(lipgloss.Color("#94e2d5")).
	Padding(0, 1)

// helpBar 把所有按键提示渲染成一行：
// 按键用彩色小块高亮，后跟灰色说明，未提供的按键不会出现
func helpBar(items ...keyHint) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, keyStyle.Render(it.key)+dimStyle.Render(" "+it.desc))
	}
	return strings.Join(parts, "  ")
}

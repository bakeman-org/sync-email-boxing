package ui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/save"
)

// 构造一封带附件的 .eml 文件
func writeTestEML(t *testing.T) string {
	t.Helper()
	payload := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 fake pdf content"))
	raw := "From: a@qq.com\r\nTo: me@qq.com\r\nSubject: 面试邀请\r\n" +
		"Date: Tue, 10 Nov 2026 03:19:28 +0000\r\n" +
		"Content-Type: multipart/mixed; boundary=XX\r\n\r\n" +
		"--XX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n你好，这是正文\r\n" +
		"--XX\r\nContent-Type: application/pdf; name=\"简历.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"简历.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" + payload + "\r\n" +
		"--XX--\r\n"
	p := filepath.Join(t.TempDir(), "面试邀请.eml")
	if err := os.WriteFile(p, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func keyD() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")} }

// 按 d 后应返回命令，并立即显示“正在保存”提示
func TestEMLViewerDPress(t *testing.T) {
	save.SetBase(t.TempDir())
	m, err := NewEMLViewer(writeTestEML(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.body.Attachments) != 1 || m.body.Attachments[0].Name != "简历.pdf" {
		t.Fatalf("附件解析错误: %+v", m.body.Attachments)
	}
	model, cmd := m.Update(keyD())
	if cmd == nil {
		t.Fatal("按 d 应返回保存命令")
	}
	if !strings.Contains(model.(EMLModel).View(), "正在保存") {
		t.Fatalf("按 d 后应显示进行中提示:\n%s", model.(EMLModel).View())
	}
}

// 保存完成消息必须显示在回显区（此前的 bug：savedMsg 未处理，界面无任何反馈）
func TestEMLViewerSavedMsgFeedback(t *testing.T) {
	save.SetBase(t.TempDir())
	m, err := NewEMLViewer(writeTestEML(t))
	if err != nil {
		t.Fatal(err)
	}
	model, _ := m.Update(savedMsg{paths: []string{"/tmp/简历.pdf"}})
	if view := model.(EMLModel).View(); !strings.Contains(view, "已保存") {
		t.Fatalf("保存成功应回显:\n%s", view)
	}

	model, _ = m.Update(savedMsg{paths: []string{"/tmp/简历.pdf"}, existing: 1})
	if view := model.(EMLModel).View(); !strings.Contains(view, "已存在") {
		t.Fatalf("重复保存应提示已存在:\n%s", view)
	}

	model, _ = m.Update(savedMsg{err: os.ErrPermission})
	if view := model.(EMLModel).View(); !strings.Contains(view, "permission") {
		t.Fatalf("保存失败应显示错误:\n%s", view)
	}
}

func TestEMLViewerNoAttachment(t *testing.T) {
	save.SetBase(t.TempDir())
	raw := "Subject: plain\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n没有附件\r\n"
	p := filepath.Join(t.TempDir(), "plain.eml")
	if err := os.WriteFile(p, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEMLViewer(p)
	if err != nil {
		t.Fatal(err)
	}
	model, _ := m.Update(keyD())
	if !strings.Contains(model.(EMLModel).View(), "没有附件") {
		t.Fatalf("无附件应有提示:\n%s", model.(EMLModel).View())
	}
}

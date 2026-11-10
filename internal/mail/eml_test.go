package mail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEMLAndHeaders(t *testing.T) {
	raw := []byte(buildGBMessage(t, gbkEnc(t, "这是 .eml 正文测试，中文应正常。")))
	f := filepath.Join(t.TempDir(), "test.eml")
	if err := os.WriteFile(f, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	sum, body, out, err := ParseEMLFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(raw) {
		t.Fatal("原始字节不一致")
	}
	if sum.Subject != "测试中文主题" {
		t.Fatalf("主题解码失败: %q", sum.Subject)
	}
	if !strings.Contains(sum.From, "张三") || !strings.Contains(sum.From, "zhangsan@qq.com") {
		t.Fatalf("发件人解码失败: %q", sum.From)
	}
	if !strings.Contains(body.Text, "中文应正常") {
		t.Fatalf("正文解码失败: %q", body.Text)
	}

	hdrs := EMLFullHeaders(raw)
	for _, want := range []string{"From: ", "To: me@qq.com", "Subject: 测试中文主题", "Content-Type: "} {
		if !strings.Contains(hdrs, want) {
			t.Fatalf("完整头信息缺少 %q:\n%s", want, hdrs)
		}
	}

	if addr := ExtractAddress(sum.From); addr != "zhangsan@qq.com" {
		t.Fatalf("提取地址失败: %q", addr)
	}
}

func TestParseEMLWithAttachment(t *testing.T) {
	payload := base64Encode([]byte{1, 2, 3, 4})
	raw := "From: =?utf-8?B?" + base64Encode([]byte("李四")) + "?= <lisi@163.com>\r\n" +
		"To: me@qq.com\r\n" +
		"Subject: =?utf-8?B?" + base64Encode([]byte("带附件")) + "?=\r\n" +
		"Date: Mon, 10 Nov 2026 08:00:00 +0800\r\n" +
		"Content-Type: multipart/mixed; boundary=XX\r\n\r\n" +
		"--XX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello 你好\r\n" +
		"--XX\r\nContent-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=\"=?utf-8?B?" + base64Encode([]byte("数据.bin")) + "?=\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" + payload + "\r\n--XX--\r\n"

	sum, body, _, err := ParseEMLFile(writeTemp(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	if sum.Subject != "带附件" {
		t.Fatalf("主题错误: %q", sum.Subject)
	}
	if len(body.Attachments) != 1 || body.Attachments[0].Name != "数据.bin" {
		t.Fatalf("附件解析错误: %+v", body.Attachments)
	}
	if string(body.Attachments[0].Data) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("附件内容错误: %v", body.Attachments[0].Data)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "a.eml")
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

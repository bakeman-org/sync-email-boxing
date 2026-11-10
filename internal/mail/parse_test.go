package mail

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// gbkEnc 把 UTF-8 字符串转成 GBK 字节
func gbkEnc(t *testing.T, s string) []byte {
	t.Helper()
	out, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// 构造一封 GBK base64 编码的邮件（模拟 QQ 邮箱客户端）
func buildGBMessage(t *testing.T, body []byte) string {
	enc := base64Encode(body)
	return "From: =?GBK?B?" + base64Encode(gbkEnc(t, "张三")) + "?= <zhangsan@qq.com>\r\n" +
		"To: me@qq.com\r\n" +
		"Subject: =?GBK?B?" + base64Encode(gbkEnc(t, "测试中文主题")) + "?=\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=GBK\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		enc + "\r\n"
}

func base64Encode(b []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb strings.Builder
	for i := 0; i < len(b); i += 3 {
		var chunk [3]byte
		copy(chunk[:], b[i:])
		n := len(b) - i
		sb.WriteByte(base64Chars[chunk[0]>>2])
		sb.WriteByte(base64Chars[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			sb.WriteByte(base64Chars[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		} else {
			sb.WriteByte('=')
		}
		if n > 2 {
			sb.WriteByte(base64Chars[chunk[2]&0x3f])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}

func TestDecodeHeaderGBK(t *testing.T) {
	got := decodeHeader("=?GBK?B?" + base64Encode(gbkEnc(t, "你好，世界")) + "?=")
	if got != "你好，世界" {
		t.Fatalf("GBK 主题解码失败: %q", got)
	}
}

func TestParseBodyGBKBase64(t *testing.T) {
	body := gbkEnc(t, "这是一封测试邮件，中文内容应当正常显示。")
	b := ParseBody([]byte(buildGBMessage(t, body)))
	if !strings.Contains(b.Text, "中文内容应当正常显示") {
		t.Fatalf("GBK 正文解码失败: %q", b.Text)
	}
}

func TestParseBodyUTF8QuotedPrintable(t *testing.T) {
	raw := "From: a@qq.com\r\nSubject: hello\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"=E4=BD=A0=E5=A5=BD=E4=B8=AD=E6=96=87\r\n"
	b := ParseBody([]byte(raw))
	if strings.TrimSpace(b.Text) != "你好中文" {
		t.Fatalf("QP UTF-8 正文解码失败: %q", b.Text)
	}
}

func TestParseBodyMultipartHTMLFallback(t *testing.T) {
	raw := "From: a@qq.com\r\nSubject: s\r\n" +
		"Content-Type: multipart/mixed; boundary=XX\r\n\r\n" +
		"--XX\r\nContent-Type: multipart/alternative; boundary=YY\r\n\r\n" +
		"--YY\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><body><p>你好 <b>世界</b></p></body></html>\r\n" +
		"--YY--\r\n" +
		"--XX\r\nContent-Type: application/pdf; name=\"file.pdf\"\r\n\r\nbinary\r\n" +
		"--XX--\r\n"
	b := ParseBody([]byte(raw))
	if !strings.Contains(b.Text, "你好 世界") {
		t.Fatalf("HTML 剥离失败: %q", b.Text)
	}
	if len(b.Attachments) != 1 || b.Attachments[0].Name != "file.pdf" {
		t.Fatalf("附件解析失败: %+v", b.Attachments)
	}
}

func TestParseAttachmentContent(t *testing.T) {
	// base64 的二进制附件内容应被解码保留
	payload := base64Encode([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
	raw := "From: a@qq.com\r\nSubject: s\r\n" +
		"Content-Type: multipart/mixed; boundary=XX\r\n\r\n" +
		"--XX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello\r\n" +
		"--XX\r\nContent-Type: application/octet-stream; name=\"data.bin\"\r\n" +
		"Content-Disposition: attachment; filename=\"data.bin\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" + payload + "\r\n" +
		"--XX--\r\n"
	b := ParseBody([]byte(raw))
	if len(b.Attachments) != 1 {
		t.Fatalf("附件数量错误: %+v", b.Attachments)
	}
	a := b.Attachments[0]
	if a.Name != "data.bin" {
		t.Fatalf("附件名错误: %q", a.Name)
	}
	want := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	if string(a.Data) != string(want) {
		t.Fatalf("附件内容错误: %v", a.Data)
	}
}

func TestParseBodyUndeclaredGBK(t *testing.T) {
	// 无 charset 声明的 GBK 字节流应回退到 GB18030 解码
	gbk := []byte{0xc4, 0xe3, 0xba, 0xc3} // "你好" in GBK
	raw := "From: a@qq.com\r\nSubject: s\r\nContent-Type: text/plain\r\n\r\n"
	raw += string(gbk)
	b := ParseBody([]byte(raw))
	if strings.TrimSpace(b.Text) != "你好" {
		t.Fatalf("GBK 回退解码失败: %q", b.Text)
	}
}

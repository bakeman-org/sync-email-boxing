package mail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"

	"mime/quotedprintable"
)

// Attachment 邮件附件（含解码后的内容）
type Attachment struct {
	Name        string `json:"name"`         // 文件名（已解码 RFC2047）
	ContentType string `json:"content_type"` // MIME 类型
	Data        []byte `json:"data"`         // 解码后的二进制内容
}

// Body 解析后的邮件正文
type Body struct {
	Text        string       `json:"text"`        // 纯文本正文（HTML 邮件会被剥离标签）
	HTML        string       `json:"html"`        // 原始 HTML 正文（如有）
	Attachments []Attachment `json:"attachments"` // 附件列表（含内容）
}

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reBr     = regexp.MustCompile(`(?i)<br\s*/?>`)
	rePEnd   = regexp.MustCompile(`(?i)</(p|div|tr|table|h[1-6])>`)
	reLi     = regexp.MustCompile(`(?i)<li[^>]*>`)
	reTag    = regexp.MustCompile(`<[^>]*>`)
	reN3     = regexp.MustCompile(`\n{3,}`)
	reLineSP = regexp.MustCompile(`(?m)^[ \t]+`)
)

// ParseBody 解析原始 RFC822 邮件，提取正文（优先 text/plain，其次 HTML）
func ParseBody(raw []byte) Body {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Body{Text: decodeBytes(raw, "")}
	}
	var b Body
	walk(msg.Header, msg.Body, &b)
	if b.Text == "" && b.HTML != "" {
		b.Text = stripHTML(b.HTML)
	}
	if b.Text == "" {
		b.Text = "(空正文)"
	}
	return b
}

func walk(header mail.Header, r io.Reader, b *Body) {
	ct := header.Get("Content-Type")
	mediaType, params := "text/plain", map[string]string{}
	if ct != "" {
		if mt, p, err := mime.ParseMediaType(ct); err == nil {
			mediaType, params = mt, p
		}
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return
		}
		mr := multipart.NewReader(r, boundary)
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			walk(mail.Header(part.Header), part, b)
		}
		return
	}

	data, err := decodeTransfer(header.Get("Content-Transfer-Encoding"), r)
	if err != nil {
		data, _ = io.ReadAll(r)
	}

	// text/* 才做字符集解码；其余视为附件，保留原始字节
	if strings.HasPrefix(mediaType, "text/") && disposition(header) != "attachment" {
		text := decodeBytes(data, params["charset"])
		switch mediaType {
		case "text/plain":
			if b.Text == "" {
				b.Text = strings.TrimSpace(text)
			}
		case "text/html":
			if b.HTML == "" {
				b.HTML = text
			}
		}
		return
	}

	if mediaType == "text/plain" || mediaType == "text/html" {
		// 声明了 attachment 的文本部分也当作附件
		text := decodeBytes(data, params["charset"])
		b.Attachments = append(b.Attachments, Attachment{
			Name:        attachmentName(header, params, mediaType),
			ContentType: mediaType,
			Data:        []byte(text),
		})
		return
	}
	b.Attachments = append(b.Attachments, Attachment{
		Name:        attachmentName(header, params, mediaType),
		ContentType: mediaType,
		Data:        data,
	})
}

// disposition 返回 Content-Disposition 的类型（inline/attachment/空）
func disposition(header mail.Header) string {
	cd := header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	if mt, p, err := mime.ParseMediaType(cd); err == nil {
		_ = p
		return strings.ToLower(mt)
	}
	return ""
}

// attachmentName 确定附件文件名：优先 Content-Disposition filename，其次 name 参数，最后按类型生成
func attachmentName(header mail.Header, params map[string]string, mediaType string) string {
	name := ""
	if cd := header.Get("Content-Disposition"); cd != "" {
		if _, p, err := mime.ParseMediaType(cd); err == nil {
			name = p["filename"]
		}
	}
	if name == "" {
		name = params["name"]
	}
	if name != "" {
		return decodeHeader(name)
	}
	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(exts) == 0 {
		return mediaType
	}
	return "attachment" + exts[0]
}

func decodeTransfer(enc string, r io.Reader) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(r))
	case "base64":
		raw, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		clean := stripWS(raw)
		if out, err := base64.StdEncoding.DecodeString(string(clean)); err == nil {
			return out, nil
		}
		return base64.RawStdEncoding.DecodeString(string(clean))
	default:
		return io.ReadAll(r)
	}
}

func stripWS(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		switch c {
		case ' ', '\t', '\r', '\n':
		default:
			out = append(out, c)
		}
	}
	return out
}

// stripHTML 把 HTML 正文转成可读纯文本
func stripHTML(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reBr.ReplaceAllString(s, "\n")
	s = rePEnd.ReplaceAllString(s, "\n")
	s = reLi.ReplaceAllString(s, "\n- ")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = reLineSP.ReplaceAllString(s, "")
	s = reN3.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// humanSize 字节数转可读大小
func humanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
	}
}

// RenderHeaders 生成查看页顶部头信息
func RenderHeaders(from, subject, date string, b Body) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "主题: %s\n", subject)
	fmt.Fprintf(&sb, "发件人: %s\n", from)
	fmt.Fprintf(&sb, "日期: %s\n", date)
	if len(b.Attachments) > 0 {
		names := make([]string, 0, len(b.Attachments))
		for _, a := range b.Attachments {
			names = append(names, fmt.Sprintf("%s(%s)", a.Name, humanSize(len(a.Data))))
		}
		fmt.Fprintf(&sb, "附件: %s\n", strings.Join(names, ", "))
	}
	sb.WriteString(strings.Repeat("─", 40))
	return sb.String()
}

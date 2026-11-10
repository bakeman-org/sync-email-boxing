package mail

import (
	"bytes"
	"io"
	"mime"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// decoderFor 返回常见中文/西文字符集的解码器
func decoderFor(charset string) encoding.Encoding {
	switch normalizeCharset(charset) {
	case "utf-8", "utf8", "ascii", "us-ascii", "":
		return unicode.UTF8
	case "gb2312", "gbk", "gb18030", "csgb2312", "csiso58gb231280", "gb_2312-80":
		return simplifiedchinese.GB18030
	case "big5", "big5-hkscs", "cn-big5", "csbig5":
		return traditionalchinese.Big5
	case "iso-8859-1", "latin1", "iso8859-1":
		return charmap.ISO8859_1
	case "windows-1252", "cp1252":
		return charmap.Windows1252
	case "utf-16", "utf-16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case "utf-16be":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	default:
		return nil
	}
}

func normalizeCharset(cs string) string {
	return strings.ToLower(strings.TrimSpace(cs))
}

// decodeHeader 解码 MIME encoded-word（=?GBK?B?...?= 之类）
func decodeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	dec := new(mime.WordDecoder)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		enc := decoderFor(charset)
		if enc == nil {
			return input, nil
		}
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		out, err := enc.NewDecoder().Bytes(data)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(out), nil
	}
	out, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}

// decodeBytes 按指定字符集把正文字节转为 UTF-8 字符串
func decodeBytes(data []byte, charset string) string {
	cs := normalizeCharset(charset)
	if cs == "" || cs == "utf-8" || cs == "utf8" || cs == "ascii" || cs == "us-ascii" {
		if utf8.Valid(data) {
			return string(data)
		}
		// 无声明且非合法 UTF-8，多半是 GBK 系
		if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data); err == nil && utf8.Valid(out) {
			return string(out)
		}
		return string(data)
	}
	if enc := decoderFor(cs); enc != nil {
		if out, err := enc.NewDecoder().Bytes(data); err == nil {
			return string(out)
		}
	}
	return string(data)
}

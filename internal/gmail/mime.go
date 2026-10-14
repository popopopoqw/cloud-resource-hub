package gmail

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"strings"

	"resource-center/internal/mail"
)

// ---- 解析辅助（纯标准库） ----

func jsonDecode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// decodeMIME 解码邮件头中的编码词（=?UTF-8?B?...?= 等）。
func decodeMIME(s string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}

// collectParts 递归收集 multipart 正文与附件元数据。
func collectParts(p *rawPayload, text, html *string, atts *[]mail.Attachment) {
	if p == nil {
		return
	}
	// 带 attachmentId 的 part 即附件
	if p.Filename != "" && p.Body != nil && p.Body.AttachmentID != "" {
		*atts = append(*atts, mail.Attachment{
			ID: p.Body.AttachmentID, Filename: p.Filename,
			MimeType: p.MimeType, Size: p.Body.Size,
		})
		return
	}
	switch {
	case strings.HasPrefix(p.MimeType, "multipart/") && len(p.Parts) > 0:
		for _, sub := range p.Parts {
			collectParts(sub, text, html, atts)
		}
	case p.MimeType == "text/plain" && p.Body != nil && *text == "":
		*text = decodeBody(p.Body.Data)
	case p.MimeType == "text/html" && p.Body != nil && *html == "":
		*html = decodeBody(p.Body.Data)
	}
}

// decodeBody 解码 base64url 编码的正文。
func decodeBody(data string) string {
	if data == "" {
		return ""
	}
	s := strings.NewReplacer("-", "+", "_", "/").Replace(data)
	for len(s)%4 != 0 {
		s += "="
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return data // 解不开时原文兜底
	}
	return string(b)
}

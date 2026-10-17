package library

import (
	"encoding/xml"
	"html"
	"io"
	"strings"
)

// PlainText handles common feed HTML with the stdlib's permissive tokenizer.
// Malformed markup falls back to literal text (still sanitized by the renderer).
func PlainText(raw string) string {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	var out strings.Builder
	hidden := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return strings.TrimSpace(html.UnescapeString(raw))
		}
		switch token := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(token.Name.Local)
			if name == "script" || name == "style" {
				hidden++
			}
			if hidden == 0 && block(name) {
				out.WriteByte('\n')
			}
		case xml.EndElement:
			name := strings.ToLower(token.Name.Local)
			if name == "script" || name == "style" {
				hidden = max(0, hidden-1)
			}
			if hidden == 0 && block(name) {
				out.WriteByte('\n')
			}
		case xml.CharData:
			if hidden == 0 {
				out.Write(token)
			}
		}
	}
	result := strings.TrimSpace(out.String())
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return result
}
func block(name string) bool {
	switch name {
	case "p", "div", "br", "li", "h1", "h2", "h3":
		return true
	}
	return false
}

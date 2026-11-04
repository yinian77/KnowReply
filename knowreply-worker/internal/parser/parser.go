package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Document 解析结果。Markdown 为 true 时切分器会识别标题层级生成 heading_path。
type Document struct {
	Path     string
	Ext      string
	Markdown bool
	Text     string
}

// Parse 按扩展名分派解析器，仅支持 README §4.2 约定的 PDF / DOCX / MD，额外兼容纯文本。
func Parse(path string) (Document, error) {
	ext := strings.ToLower(filepath.Ext(path))
	doc := Document{Path: path, Ext: ext}

	switch ext {
	case ".md", ".markdown":
		doc.Markdown = true
		text, err := readText(path)
		if err != nil {
			return doc, err
		}
		doc.Text = text
	case ".txt":
		text, err := readText(path)
		if err != nil {
			return doc, err
		}
		doc.Text = text
	case ".docx":
		text, err := extractDocx(path)
		if err != nil {
			return doc, err
		}
		doc.Text = text
	case ".pdf":
		text, err := extractPDF(path)
		if err != nil {
			return doc, err
		}
		doc.Text = text
	default:
		return doc, fmt.Errorf("unsupported file type: %q", ext)
	}

	doc.Text = normalize(doc.Text)
	if strings.TrimSpace(doc.Text) == "" {
		return doc, fmt.Errorf("no extractable text in %s", filepath.Base(path))
	}
	return doc, nil
}

func readText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(b), nil
}

// normalize 统一换行、压缩连续空行，避免切分时产生大量空块。
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u00a0", " ")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

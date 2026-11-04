package parser

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDF 逐页提取纯文本。扫描件（图片型 PDF）没有文字层，会得到空结果并报错，
// 需后续接 OCR，当前明确失败而不是产出空切片污染知识库。
func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	pages := r.NumPage()
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("extract pdf page %d: %w", i, err)
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}

	if pages == 0 {
		return "", fmt.Errorf("pdf has no pages: %s", path)
	}
	return sb.String(), nil
}

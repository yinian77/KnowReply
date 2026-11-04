package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractDocx 直接读 OOXML（word/document.xml），不引第三方库：
// <w:p> 段落边界转 \n，<w:t> 内文本拼接。
func extractDocx(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer zr.Close()

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("word/document.xml not found in %s", path)
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("open word/document.xml: %w", err)
	}
	defer rc.Close()

	text, err := docxText(rc)
	if err != nil {
		return "", err
	}
	return text, nil
}

func docxText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false

	var (
		sb       strings.Builder
		inText   bool
		inPara   bool
		tableRow bool
	)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("decode docx xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "p":
				inPara = true
			case "tr":
				tableRow = true
			case "tab":
				sb.WriteString("\t")
			case "br":
				sb.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				if inPara {
					sb.WriteString("\n")
					inPara = false
				}
			case "tr":
				if tableRow {
					sb.WriteString("\n")
					tableRow = false
				}
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}

	return sb.String(), nil
}

package chunker

import (
	"strings"
	"unicode/utf8"
)

// Chunk 一个检索单元。Content 已带上标题路径前缀，HeadingPath 单独保留用于展示与过滤。
type Chunk struct {
	HeadingPath string
	Content     string
}

// Splitter 按「先按标题分节、再按段落聚合到 300~600 字」的策略切分。
type Splitter struct {
	MinRunes int
	MaxRunes int
}

type section struct {
	heading string
	body    []string
}

// Split 切分文本；markdown 为 true 时识别 # 标题层级生成 heading_path。
func (s Splitter) Split(text string, markdown bool) []Chunk {
	if s.MinRunes <= 0 {
		s.MinRunes = 300
	}
	if s.MaxRunes < s.MinRunes {
		s.MaxRunes = 600
	}

	sections := splitSections(text, markdown)
	out := make([]Chunk, 0, len(sections))
	for _, sec := range sections {
		out = append(out, s.chunkSection(sec)...)
	}
	return out
}

// splitSections 把文本切成「标题 + 正文段落」，Markdown 下维护标题栈。
func splitSections(text string, markdown bool) []section {
	lines := strings.Split(text, "\n")

	if !markdown {
		return []section{{heading: "", body: paragraphs(lines)}}
	}

	var (
		sections []section
		stack    []string // 标题栈，下标对应层级
		current  []string
	)

	flush := func() {
		if len(current) == 0 {
			return
		}
		// current 是原始行（含空行），必须转成段落，否则会产生空切片
		body := paragraphs(current)
		if len(body) == 0 {
			current = nil
			return
		}
		sections = append(sections, section{heading: strings.Join(stack, " > "), body: body})
		current = nil
	}

	for _, line := range lines {
		level, title, ok := parseHeading(line)
		if !ok {
			current = append(current, line)
			continue
		}

		flush()
		if level <= len(stack) {
			stack = stack[:level-1]
		} else {
			for len(stack) < level-1 {
				stack = append(stack, "")
			}
		}
		stack = append(stack, title)
		// 清理空层级，避免出现 " >  > 标题"
		cleaned := make([]string, 0, len(stack))
		for _, h := range stack {
			if h != "" {
				cleaned = append(cleaned, h)
			}
		}
		stack = cleaned
	}

	flush()
	if len(sections) == 0 {
		return []section{{heading: "", body: paragraphs(lines)}}
	}
	return sections
}

// parseHeading 识别 "# 标题" 形式的 ATX 标题。
func parseHeading(line string) (level int, title string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return 0, "", false
	}
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}

	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(trimmed) {
		return 0, "", false
	}
	if trimmed[n] != ' ' {
		return 0, "", false
	}

	title = strings.TrimSpace(trimmed[n+1:])
	if title == "" {
		return 0, "", false
	}
	return n, title, true
}

// paragraphs 按空行聚合段落，顺带丢弃空行。
func paragraphs(lines []string) []string {
	out := make([]string, 0, len(lines))
	var buf []string

	flush := func() {
		if len(buf) == 0 {
			return
		}
		out = append(out, strings.Join(buf, "\n"))
		buf = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return out
}

// chunkSection 把一节的段落聚合到目标长度区间。
func (s Splitter) chunkSection(sec section) []Chunk {
	prefix := ""
	if sec.heading != "" {
		prefix = sec.heading + "\n"
	}
	prefixLen := utf8.RuneCountInString(prefix)

	var (
		out  []Chunk
		buf  []string
		size int
	)

	emit := func() {
		if len(buf) == 0 {
			return
		}
		out = append(out, Chunk{
			HeadingPath: sec.heading,
			Content:     prefix + strings.Join(buf, "\n\n"),
		})
		buf = nil
		size = 0
	}

	for _, p := range sec.body {
		p = strings.TrimSpace(p)
		pLen := utf8.RuneCountInString(p)
		if pLen == 0 {
			continue
		}

		// 单段超长：先冲掉缓冲，再把长段按句子拆开
		if pLen+prefixLen > s.MaxRunes {
			emit()
			for _, piece := range s.splitLong(p, s.MaxRunes-prefixLen) {
				out = append(out, Chunk{HeadingPath: sec.heading, Content: prefix + piece})
			}
			continue
		}

		// 加入本段会超上限，且缓冲已达下限：先冲掉
		if size > 0 && size+pLen > s.MaxRunes {
			emit()
		}

		buf = append(buf, p)
		size += pLen
		if len(buf) > 1 {
			size += 2 // "\n\n"
		}

		// 达到下限后继续累积，直到接近上限再冲，避免产生大量过小切片
		if size >= s.MinRunes && size >= s.MaxRunes*8/10 {
			emit()
		}
	}

	emit()
	return out
}

// splitLong 把超长段落按句末标点拆成不超过 limit 的片段，仍超长则硬切。
func (s Splitter) splitLong(text string, limit int) []string {
	if limit < 50 {
		limit = 50
	}

	var (
		out  []string
		buf  strings.Builder
		size int
	)

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		out = append(out, buf.String())
		buf.Reset()
		size = 0
	}

	for _, sentence := range splitSentences(text) {
		sLen := utf8.RuneCountInString(sentence)
		if sLen > limit {
			flush()
			out = append(out, hardSplit(sentence, limit)...)
			continue
		}
		if size+sLen > limit {
			flush()
		}
		buf.WriteString(sentence)
		size += sLen
	}
	flush()
	return out
}

// splitSentences 按中英文句末标点断句，标点保留在前一句末尾。
func splitSentences(text string) []string {
	var (
		out []string
		buf strings.Builder
	)
	for _, r := range text {
		buf.WriteRune(r)
		switch r {
		case '。', '！', '？', '；', '\n', '.', '!', '?', ';':
			out = append(out, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

// hardSplit 无标点长文本的兜底：按 rune 数硬切。
func hardSplit(text string, limit int) []string {
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}
	out := make([]string, 0, len(runes)/limit+1)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
	}
	return out
}

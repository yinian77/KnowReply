package chunker

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMarkdownKeepsHeadingPath(t *testing.T) {
	text := `# 售后

## 退款

退款将在审核通过后 3 个工作日内到账。若超时未到账，请联系人工客服核对支付渠道。

## 发票

电子发票在订单完成后自动开具，可在订单详情页下载。

# 物流

物流信息以承运商回传为准，通常发货后 24 小时内更新。
`
	sp := Splitter{MinRunes: 20, MaxRunes: 120}
	chunks := sp.Split(text, true)

	if len(chunks) == 0 {
		t.Fatal("expect chunks, got none")
	}
	seen := map[string]bool{}
	for _, c := range chunks {
		if c.HeadingPath == "" {
			t.Fatalf("markdown chunk should carry heading path, got %+v", c)
		}
		if !strings.HasPrefix(c.Content, c.HeadingPath+"\n") {
			t.Fatalf("content should be prefixed by heading path, got %q", c.Content)
		}
		if utf8.RuneCountInString(c.Content) > 120 {
			t.Fatalf("chunk exceeds max runes: %d", utf8.RuneCountInString(c.Content))
		}
		seen[c.HeadingPath] = true
	}

	for _, want := range []string{"售后 > 退款", "售后 > 发票", "物流"} {
		if !seen[want] {
			t.Fatalf("missing heading path %q, got %v", want, seen)
		}
	}
}

// 回归：只有标题、没有正文的章节不得产出空内容切片（真实文档里很常见）。
func TestSplitMarkdownSkipsHeadingOnlySection(t *testing.T) {
	text := `# 售后服务

## 退款到账时间

退款将在审核通过后 3 个工作日内到账。

# 物流与配送

## 发货时效

现货商品付款后 48 小时内发出。
`
	sp := Splitter{MinRunes: 20, MaxRunes: 600}
	chunks := sp.Split(text, true)

	if len(chunks) != 2 {
		t.Fatalf("expect 2 chunks, got %d: %+v", len(chunks), chunks)
	}
	for _, c := range chunks {
		body := strings.TrimPrefix(c.Content, c.HeadingPath)
		if strings.TrimSpace(body) == "" {
			t.Fatalf("chunk has empty body: %q", c.Content)
		}
	}
}

func TestSplitLongParagraphRespectsMax(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		sb.WriteString("这是一句用于测试切分的较长句子，需要重复多次。")
	}

	sp := Splitter{MinRunes: 50, MaxRunes: 100}
	chunks := sp.Split(sb.String(), false)

	if len(chunks) < 2 {
		t.Fatalf("long text should be split into multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if n := utf8.RuneCountInString(c.Content); n > 100 {
			t.Fatalf("chunk exceeds max runes: %d", n)
		}
		if c.HeadingPath != "" {
			t.Fatalf("plain text chunk should have empty heading, got %q", c.HeadingPath)
		}
	}
}

func TestSplitPlainTextWithoutHeadings(t *testing.T) {
	text := "第一段内容，讲的是退款时效问题。\n\n第二段内容，讲的是发票开具方式。\n"
	sp := Splitter{MinRunes: 10, MaxRunes: 200}

	chunks := sp.Split(text, false)
	if len(chunks) != 1 {
		t.Fatalf("expect 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "退款时效") || !strings.Contains(chunks[0].Content, "发票开具") {
		t.Fatalf("chunk should contain both paragraphs, got %q", chunks[0].Content)
	}
}

func TestParseHeading(t *testing.T) {
	cases := []struct {
		line   string
		level  int
		title  string
		isHead bool
	}{
		{"# 一级", 1, "一级", true},
		{"### 三级标题", 3, "三级标题", true},
		{"####### 七级", 0, "", false},
		{"#没有空格", 0, "", false},
		{"#", 0, "", false},
		{"正文内容", 0, "", false},
	}
	for _, c := range cases {
		level, title, ok := parseHeading(c.line)
		if ok != c.isHead || level != c.level || title != c.title {
			t.Fatalf("parseHeading(%q) = (%d,%q,%v), want (%d,%q,%v)",
				c.line, level, title, ok, c.level, c.title, c.isHead)
		}
	}
}

package retrieval

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/embed"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

// rrfK RRF 平滑常数，取 60 是通用经验值。
const rrfK = 60

// Request 检索请求。各路 TopK 是单路召回数量，Limit 是融合后返回条数。
type Request struct {
	TenantID   uint64
	Query      string
	VectorTopK int
	TextTopK   int
	GramTopK   int
	Limit      int
}

// Hit 融合后的命中项。某路 Rank 为 0 表示该路未召回。
type Hit struct {
	ChunkID     int64   `json:"chunkId"`
	DocID       uint64  `json:"docId"`
	HeadingPath string  `json:"headingPath"`
	Content     string  `json:"content"`
	VectorScore float64 `json:"vectorScore"`
	TextScore   float64 `json:"textScore"`
	GramScore   float64 `json:"gramScore"`
	FusedScore  float64 `json:"fusedScore"`
	VectorRank  int     `json:"vectorRank"`
	TextRank    int     `json:"textRank"`
	GramRank    int     `json:"gramRank"`
}

// Response 附带各路召回规模，便于判断是哪一路在起作用。
type Response struct {
	Query      string `json:"query"`
	VectorHits int    `json:"vectorHits"`
	TextHits   int    `json:"textHits"`
	GramHits   int    `json:"gramHits"`
	Embedder   string `json:"embedder"`
	Hits       []Hit  `json:"hits"`
}

type Searcher struct {
	kb  *store.PG
	emb embed.Embedder
	log *slog.Logger
	// vectorWeight 向量路的 RRF 权重。mock 向量是确定性噪声，
	// 权重压低到 0.2 让词面证据主导排序，避免预览里出现「答非所问排第一」的假象。
	vectorWeight float64
}

func NewSearcher(kb *store.PG, emb embed.Embedder, log *slog.Logger) *Searcher {
	weight := 1.0
	if strings.HasPrefix(emb.Describe(), "mock") {
		weight = 0.2
	}
	return &Searcher{kb: kb, emb: emb, log: log, vectorWeight: weight}
}

func (s *Searcher) Search(ctx context.Context, req Request) (*Response, error) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if req.TenantID == 0 {
		return nil, fmt.Errorf("tenantId is required")
	}
	if req.VectorTopK <= 0 {
		req.VectorTopK = 20
	}
	if req.TextTopK <= 0 {
		req.TextTopK = 20
	}
	if req.GramTopK <= 0 {
		req.GramTopK = 20
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}

	vecs, err := s.emb.Embed(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embed query: expect 1 vector, got %d", len(vecs))
	}

	vecHits, err := s.kb.SearchVector(ctx, req.TenantID, vecs[0], req.VectorTopK)
	if err != nil {
		return nil, err
	}
	textHits, err := s.kb.SearchFullText(ctx, req.TenantID, req.Query, req.TextTopK)
	if err != nil {
		return nil, err
	}
	// 中文二元组字面召回：SQL 内已按覆盖率排序，无需二次重排
	gramHits, err := s.kb.SearchByGrams(ctx, req.TenantID, req.Query, bigrams(req.Query), req.GramTopK)
	if err != nil {
		return nil, err
	}

	hits := fuse(vecHits, textHits, gramHits, s.vectorWeight, req.Limit)
	return &Response{
		Query:      req.Query,
		VectorHits: len(vecHits),
		TextHits:   len(textHits),
		GramHits:   len(gramHits),
		Embedder:   s.emb.Describe(),
		Hits:       hits,
	}, nil
}

// fuse 做 RRF 融合：score = Σ 1/(rrfK + rank)，rank 从 1 开始。
func fuse(vecHits, textHits, gramHits []store.Scored, vectorWeight float64, limit int) []Hit {
	index := make(map[int64]*Hit, len(vecHits)+len(textHits)+len(gramHits))

	upsert := func(id int64, build func(*Hit)) *Hit {
		if h, ok := index[id]; ok {
			return h
		}
		h := &Hit{ChunkID: id}
		build(h)
		index[id] = h
		return h
	}

	for i, r := range vecHits {
		rank := i + 1
		h := upsert(r.ID, func(h *Hit) {
			h.DocID = r.DocID
			h.Content = r.Content
			h.HeadingPath = r.HeadingPath
		})
		h.VectorScore = r.Score
		h.VectorRank = rank
		h.FusedScore += vectorWeight / float64(rrfK+rank)
	}

	for i, r := range textHits {
		rank := i + 1
		h := upsert(r.ID, func(h *Hit) {
			h.DocID = r.DocID
			h.Content = r.Content
			h.HeadingPath = r.HeadingPath
		})
		h.TextScore = r.Score
		h.TextRank = rank
		h.FusedScore += 1.0 / float64(rrfK+rank)
	}

	for i, r := range gramHits {
		rank := i + 1
		h := upsert(r.ID, func(h *Hit) {
			h.DocID = r.DocID
			h.Content = r.Content
			h.HeadingPath = r.HeadingPath
		})
		h.GramScore = r.Score
		h.GramRank = rank
		h.FusedScore += 1.0 / float64(rrfK+rank)
	}

	out := make([]Hit, 0, len(index))
	for _, h := range index {
		h.Content = truncate(h.Content, 400)
		out = append(out, *h)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].FusedScore == out[j].FusedScore {
			return out[i].ChunkID < out[j].ChunkID
		}
		return out[i].FusedScore > out[j].FusedScore
	})

	if limit > len(out) {
		limit = len(out)
	}
	return out[:limit]
}

// bigrams 取查询的相邻二元组，过滤空白与标点。
// 中文没有空格分词，二元组既做召回条件也做覆盖率打分，比 tsv/similarity 更贴合短问句。
func bigrams(query string) []string {
	var runes []rune
	for _, r := range query {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		runes = append(runes, r)
	}
	if len(runes) < 2 {
		return nil
	}
	out := make([]string, 0, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		out = append(out, string(runes[i:i+2]))
	}
	return out
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

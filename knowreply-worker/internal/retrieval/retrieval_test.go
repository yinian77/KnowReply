package retrieval

import (
	"testing"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

func TestBigramsFiltersPunctuation(t *testing.T) {
	got := bigrams("退款 几天到账？")
	want := []string{"退款", "款几", "几天", "天到", "到账"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	if bigrams("好") != nil {
		t.Fatal("single rune query should produce no bigrams")
	}
}

func TestFuseOrdersByWeightedRRF(t *testing.T) {
	// 切片 1 只有词面命中，切片 2 只有向量命中
	vecHits := []store.Scored{{ID: 2, DocID: 9, Content: "b"}}
	gramHits := []store.Scored{{ID: 1, DocID: 9, Content: "a"}}

	// 向量权重 1.0 时两者 RRF 相同，按 ChunkID 稳定排序
	equal := fuse(vecHits, nil, gramHits, 1.0, 5)
	if len(equal) != 2 {
		t.Fatalf("expect 2 hits, got %d", len(equal))
	}
	if equal[0].ChunkID != 1 || equal[1].ChunkID != 2 {
		t.Fatalf("tie should fall back to chunk id asc, got %d,%d", equal[0].ChunkID, equal[1].ChunkID)
	}

	// 权重压到 0.2 时词面命中的切片 1 应该胜出
	lexicalFirst := fuse(vecHits, nil, gramHits, 0.2, 5)
	if lexicalFirst[0].ChunkID != 1 {
		t.Fatalf("lexical hit should rank first, got chunk %d", lexicalFirst[0].ChunkID)
	}
	if lexicalFirst[0].GramRank != 1 || lexicalFirst[1].VectorRank != 1 {
		t.Fatalf("ranks not populated: %+v %+v", lexicalFirst[0], lexicalFirst[1])
	}
}

func TestFuseRespectsLimitAndTruncates(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "退款"
	}
	hits := fuse(nil, nil, []store.Scored{{ID: 1, DocID: 1, Content: long}}, 1.0, 1)

	if len(hits) != 1 {
		t.Fatalf("expect 1 hit, got %d", len(hits))
	}
	if n := []rune(hits[0].Content); len(n) > 401 {
		t.Fatalf("content should be truncated to ~400 runes, got %d", len(n))
	}
}

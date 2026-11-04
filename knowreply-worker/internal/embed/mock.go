package embed

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// mock 本地确定性向量，仅用于无 API Key 时的链路联调。
// 语义无意义（相似文本不会靠近），但能验证「切分 → 向量 → 落库 → 检索」全链路。
type mock struct {
	dim int
}

func newMock(dim int) *mock {
	return &mock{dim: dim}
}

func (m *mock) Dim() int { return m.dim }

func (m *mock) Describe() string { return fmt.Sprintf("mock(dim=%d)", m.dim) }

func (m *mock) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = m.vector(t)
	}
	return out, nil
}

// vector 用 sha256 分块扩展生成 dim 维向量并做 L2 归一化。
// 每 8 维消耗一个摘要（32 字节 = 8 个 uint32），同一文本输出恒定。
func (m *mock) vector(text string) []float32 {
	vec := make([]float32, m.dim)

	var norm float64
	for i := 0; i < m.dim; i += 8 {
		h := sha256.Sum256([]byte(text + "#" + strconv.Itoa(i/8)))
		for j := 0; j < 8 && i+j < m.dim; j++ {
			raw := binary.LittleEndian.Uint32(h[j*4 : j*4+4])
			v := float64(raw)/float64(math.MaxUint32)*2 - 1
			vec[i+j] = float32(v)
			norm += v * v
		}
	}

	if norm > 0 {
		inv := float32(1 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

package embed

import (
	"context"
	"fmt"
	"time"
)

// Embedder 向量化接口。实现需保证同一文本输出稳定，便于幂等重跑。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
	Describe() string
}

type Options struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Dim      int
	Batch    int
	Timeout  time.Duration
}

func New(opts Options) (Embedder, error) {
	if opts.Dim <= 0 {
		return nil, fmt.Errorf("embedding dim must be positive, got %d", opts.Dim)
	}
	if opts.Batch <= 0 {
		opts.Batch = 16
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	switch opts.Provider {
	case "", "mock":
		return newMock(opts.Dim), nil
	case "openai":
		return newOpenAI(opts), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %q", opts.Provider)
	}
}

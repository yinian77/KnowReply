package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAI 兼容 /v1/embeddings 协议（OpenAI、通义、DeepSeek 代理等均可复用）。
type openAI struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	dim     int
	batch   int
}

func newOpenAI(opts Options) *openAI {
	return &openAI{
		client:  &http.Client{Timeout: opts.Timeout},
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		apiKey:  opts.APIKey,
		model:   opts.Model,
		dim:     opts.Dim,
		batch:   opts.Batch,
	}
}

func (o *openAI) Dim() int { return o.dim }

func (o *openAI) Describe() string { return fmt.Sprintf("openai(%s)", o.model) }

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (o *openAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))

	for start := 0; start < len(texts); start += o.batch {
		end := start + o.batch
		if end > len(texts) {
			end = len(texts)
		}

		vecs, err := o.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (o *openAI) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingsRequest{Model: o.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embedding api: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var parsed embeddingsResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response (status=%d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(payload)
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("embedding api status=%d: %s", resp.StatusCode, msg)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: sent %d, got %d", len(texts), len(parsed.Data))
	}

	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, fmt.Errorf("embedding index out of range: %d", d.Index)
		}
		if len(d.Embedding) != o.dim {
			return nil, fmt.Errorf("embedding dim mismatch: want %d, got %d", o.dim, len(d.Embedding))
		}
		out[d.Index] = d.Embedding
	}

	return out, nil
}

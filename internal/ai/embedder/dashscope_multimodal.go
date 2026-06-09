package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

const defaultDashScopeMultimodalEmbeddingEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"

type dashScopeMultimodalConfig struct {
	APIKey    string
	Model     string
	Endpoint  string
	Dimension int
	Timeout   time.Duration
}

type dashScopeMultimodalEmbedder struct {
	apiKey    string
	model     string
	endpoint  string
	dimension int
	client    *http.Client
}

type multimodalEmbeddingRequest struct {
	Model      string                        `json:"model"`
	Input      multimodalEmbeddingInput      `json:"input"`
	Parameters multimodalEmbeddingParameters `json:"parameters,omitempty"`
}

type multimodalEmbeddingInput struct {
	Contents []multimodalEmbeddingContent `json:"contents"`
}

type multimodalEmbeddingContent struct {
	Text string `json:"text"`
}

type multimodalEmbeddingParameters struct {
	Dimension int `json:"dimension,omitempty"`
}

type multimodalEmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func newDashScopeMultimodalEmbedder(ctx context.Context, cfg dashScopeMultimodalConfig) (embedding.Embedder, error) {
	_ = ctx
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("embedding_model.api_key is empty")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding_model.model is empty")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultDashScopeMultimodalEmbeddingEndpoint
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &dashScopeMultimodalEmbedder{
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		endpoint:  cfg.Endpoint,
		dimension: cfg.Dimension,
		client:    &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (e *dashScopeMultimodalEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	_ = opts
	if len(texts) == 0 {
		return nil, nil
	}
	contents := make([]multimodalEmbeddingContent, 0, len(texts))
	for _, text := range texts {
		contents = append(contents, multimodalEmbeddingContent{Text: text})
	}
	reqBody := multimodalEmbeddingRequest{
		Model: e.model,
		Input: multimodalEmbeddingInput{Contents: contents},
	}
	if e.dimension > 0 {
		reqBody.Parameters.Dimension = e.dimension
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-DataInspection", "disable")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope multimodal embedding status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var payload multimodalEmbeddingResponse
	if err = json.Unmarshal(respBytes, &payload); err != nil {
		return nil, err
	}
	if payload.Code != "" {
		return nil, fmt.Errorf("dashscope multimodal embedding error code=%s message=%s request_id=%s", payload.Code, payload.Message, payload.RequestID)
	}
	if len(payload.Output.Embeddings) != len(texts) {
		return nil, fmt.Errorf("dashscope multimodal embedding result length mismatch: want=%d got=%d", len(texts), len(payload.Output.Embeddings))
	}

	vectors := make([][]float64, 0, len(payload.Output.Embeddings))
	for _, item := range payload.Output.Embeddings {
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}

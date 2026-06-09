package embedder

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/gogf/gf/v2/frame/g"
)

func DoubaoEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	model, err := g.Cfg().Get(ctx, "embedding_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "embedding_model.api_key")
	if err != nil {
		return nil, err
	}
	dim := embeddingDimension(ctx)
	if shouldUseDashScopeMultimodal(model.String()) {
		return newDashScopeMultimodalEmbedder(ctx, dashScopeMultimodalConfig{
			APIKey:    api_key.String(),
			Model:     model.String(),
			Endpoint:  cfgString(ctx, "embedding_model.endpoint"),
			Dimension: dim,
			Timeout:   30 * time.Second,
		})
	}
	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      model.String(),
		APIKey:     api_key.String(),
		Dimensions: &dim,
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return nil, err
	}
	return embedder, nil
}

func embeddingDimension(ctx context.Context) int {
	v, err := g.Cfg().Get(ctx, "embedding_model.dimension")
	if err != nil {
		return 2048
	}
	dim := v.Int()
	if dim <= 0 {
		return 2048
	}
	return dim
}

func shouldUseDashScopeMultimodal(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "multimodal") || strings.Contains(normalized, "vision")
}

func cfgString(ctx context.Context, key string) string {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v.String())
}

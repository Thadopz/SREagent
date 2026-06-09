package chat_pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

const defaultRerankTopN = 3
const dashScopeRerankEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank"

type dashScopeRerankRequest struct {
	Model      string                  `json:"model"`
	Input      dashScopeRerankInput    `json:"input"`
	Parameters dashScopeRerankParamter `json:"parameters"`
}

type dashScopeRerankInput struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type dashScopeRerankParamter struct {
	ReturnDocuments bool `json:"return_documents"`
}

type dashScopeRerankResponse struct {
	Output struct {
		Results []struct {
			Index int `json:"index"`
		} `json:"results"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func rerankDocumentsLambda(ctx context.Context, input map[string]any, opts ...any) (output map[string]any, err error) {
	query := strings.TrimSpace(toString(input["query"]))
	docs := toDocuments(input["documents"])
	if len(docs) == 0 || query == "" {
		return map[string]any{"documents": docs}, nil
	}

	topN := getRerankTopN(ctx)
	ranked, err := rerankByDashScope(ctx, query, docs, topN)
	if err != nil {
		// 降级：远程重排失败时，保留原始召回结果，避免整条链路失败。
		g.Log().Warningf(ctx, "rerank failed, fallback to original docs, err=%v", err)
		return map[string]any{"documents": docs}, nil
	}
	if shouldObserveRerankLog(ctx) {
		g.Log().Infof(ctx, "[rerank] topN=%d query=%s original=%v reranked=%v", topN, truncateForLog(query, 80), summarizeDocsForLog(docs), summarizeDocsForLog(ranked))
	}
	return map[string]any{"documents": ranked}, nil
}

func rerankByDashScope(ctx context.Context, query string, docs []*schema.Document, topN int) ([]*schema.Document, error) {
	apiKey, endpoint, err := getDashScopeRerankConfig(ctx)
	if err != nil {
		return nil, err
	}

	docContents := make([]string, 0, len(docs))
	for _, d := range docs {
		if d == nil {
			docContents = append(docContents, "")
			continue
		}
		docContents = append(docContents, d.Content)
	}

	reqBody := dashScopeRerankRequest{
		Model: "gte-rerank-v2",
		Input: dashScopeRerankInput{
			Query:     query,
			Documents: docContents,
		},
		Parameters: dashScopeRerankParamter{
			ReturnDocuments: false,
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-DashScope-DataInspection", "disable")

	httpCli := &http.Client{Timeout: 20 * time.Second}
	httpResp, err := httpCli.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope rerank status=%d body=%s", httpResp.StatusCode, string(respBytes))
	}

	var resp dashScopeRerankResponse
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "" {
		return nil, fmt.Errorf("dashscope rerank error code=%s message=%s", resp.Code, resp.Message)
	}

	type scoredDoc struct {
		doc   *schema.Document
		score int
	}
	scored := make([]scoredDoc, 0, len(docs))
	for idx, doc := range docs {
		scored = append(scored, scoredDoc{doc: doc, score: -idx})
	}
	for rank, result := range resp.Output.Results {
		if result.Index < 0 || result.Index >= len(scored) {
			continue
		}
		// 按返回顺序打分：越靠前分越高。
		scored[result.Index].score = len(scored) - rank
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if topN <= 0 {
		topN = defaultRerankTopN
	}
	if len(scored) < topN {
		topN = len(scored)
	}
	ranked := make([]*schema.Document, 0, topN)
	for i := 0; i < topN; i++ {
		ranked = append(ranked, scored[i].doc)
	}
	return ranked, nil
}

func getDashScopeRerankConfig(ctx context.Context) (apiKey string, endpoint string, err error) {
	apiKeyValue, err := g.Cfg().Get(ctx, "embedding_model.api_key")
	if err != nil {
		return "", "", err
	}
	apiKey = strings.TrimSpace(apiKeyValue.String())
	if apiKey == "" {
		return "", "", fmt.Errorf("embedding_model.api_key is empty")
	}

	baseURLValue, err := g.Cfg().Get(ctx, "embedding_model.base_url")
	if err != nil {
		return "", "", err
	}
	baseURL := strings.TrimSpace(baseURLValue.String())
	if baseURL == "" {
		return apiKey, dashScopeRerankEndpoint, nil
	}

	trimmed := strings.TrimSuffix(baseURL, "/")
	trimmed = strings.TrimSuffix(trimmed, "/compatible-mode/v1")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return apiKey, trimmed + "/api/v1/services/rerank/text-rerank/text-rerank", nil
	}
	return apiKey, dashScopeRerankEndpoint, nil
}

func getRerankTopN(ctx context.Context) int {
	v, err := g.Cfg().Get(ctx, "rerank.top_n")
	if err != nil {
		return defaultRerankTopN
	}
	n := v.Int()
	if n <= 0 {
		return defaultRerankTopN
	}
	return n
}

func shouldObserveRerankLog(ctx context.Context) bool {
	v, err := g.Cfg().Get(ctx, "rerank.observe_log")
	if err != nil {
		return false
	}
	return v.Bool()
}

func summarizeDocsForLog(docs []*schema.Document) []string {
	res := make([]string, 0, len(docs))
	for i, d := range docs {
		if d == nil {
			res = append(res, strconv.Itoa(i)+":nil")
			continue
		}
		source := "unknown"
		if d.MetaData != nil {
			if raw, ok := d.MetaData["_source"]; ok {
				source = truncateForLog(toString(raw), 40)
			}
		}
		res = append(res, strconv.Itoa(i)+":"+source+"#"+truncateForLog(d.Content, 24))
	}
	return res
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func toDocuments(v any) []*schema.Document {
	switch docs := v.(type) {
	case []*schema.Document:
		return docs
	case []schema.Document:
		res := make([]*schema.Document, 0, len(docs))
		for i := range docs {
			d := docs[i]
			res = append(res, &d)
		}
		return res
	case []any:
		res := make([]*schema.Document, 0, len(docs))
		for _, item := range docs {
			if d, ok := item.(*schema.Document); ok {
				res = append(res, d)
			}
			if d, ok := item.(schema.Document); ok {
				doc := d
				res = append(res, &doc)
			}
		}
		return res
	default:
		return nil
	}
}

package tools

import (
	"SREagent/internal/ai/retriever"
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type QueryInternalDocsInput struct {
	Query string `json:"query" jsonschema:"description=The query string to search in internal documentation for relevant information and processing steps"`
}

func NewQueryInternalDocsTool() (tool.InvokableTool, error) {
	t, err := utils.InferOptionableTool(
		"query_internal_docs",
		"Use this tool to search internal documentation and knowledge base for relevant information. It performs RAG (Retrieval-Augmented Generation) to find similar documents and extract processing steps. This is useful when you need to understand internal procedures, best practices, or step-by-step guides stored in the company's documentation.",
		func(ctx context.Context, input *QueryInternalDocsInput, opts ...tool.Option) (output string, err error) {
			rr, err := retriever.NewMilvusRetriever(ctx)
			if err != nil {
				return marshalToolError(err, "create milvus retriever failed"), nil
			}
			resp, err := rr.Retrieve(ctx, input.Query)
			if err != nil {
				return marshalToolError(err, "retrieve internal docs failed"), nil
			}
			if resp == nil {
				return marshalToolData([]any{}, "no related internal docs found"), nil
			}
			return marshalToolData(resp, fmt.Sprintf("retrieved %d internal docs", len(resp))), nil
		})
	if err != nil {
		return nil, err
	}
	return t, nil
}

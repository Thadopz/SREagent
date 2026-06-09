package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type RetrievedDocEvidence struct {
	Index   int            `json:"index"`
	Source  string         `json:"source,omitempty"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
	Stage   string         `json:"stage"`
}

type RetrievalObservation struct {
	Triggered bool                   `json:"triggered"`
	Query     string                 `json:"query"`
	Docs      []RetrievedDocEvidence `json:"docs,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

type ChatAgentOptions struct {
	ChatModel    model.ToolCallingChatModel
	Retriever    retriever.Retriever
	Tools        []tool.BaseTool
	SkipRerank   bool
	OnRetrieval  func(RetrievalObservation)
	ContextNow   func() string
	DocumentMax  int
	DocumentsMax int
	ReactMaxStep int
}

func (o ChatAgentOptions) documentMax() int {
	if o.DocumentMax > 0 {
		return o.DocumentMax
	}
	return DocumentMaxChars
}

func (o ChatAgentOptions) documentsMax() int {
	if o.DocumentsMax > 0 {
		return o.DocumentsMax
	}
	return DocumentsMaxChars
}

func documentEvidence(docs []*schema.Document, stage string) []RetrievedDocEvidence {
	evidence := make([]RetrievedDocEvidence, 0, len(docs))
	for i, doc := range docs {
		if doc == nil {
			continue
		}
		source := ""
		if doc.MetaData != nil {
			if raw, ok := doc.MetaData["_source"]; ok {
				source = toString(raw)
			}
		}
		evidence = append(evidence, RetrievedDocEvidence{
			Index:   i,
			Source:  source,
			Content: doc.Content,
			Meta:    doc.MetaData,
			Stage:   stage,
		})
	}
	return evidence
}

func emitRetrieval(ctx context.Context, opts ChatAgentOptions, obs RetrievalObservation) {
	if opts.OnRetrieval != nil {
		opts.OnRetrieval(obs)
	}
}

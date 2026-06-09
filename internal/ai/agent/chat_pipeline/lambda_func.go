package chat_pipeline

import (
	"SREagent/internal/ai/project_context"
	"SREagent/internal/ai/skills"
	"context"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func newContextDocumentsLambda(ctx context.Context, input *UserMessage, opts ...any) (output map[string]any, err error) {
	return newContextDocumentsLambdaWithOptions(ChatAgentOptions{})(ctx, input, opts...)
}

func newContextDocumentsLambdaWithOptions(agentOpts ChatAgentOptions) func(context.Context, *UserMessage, ...any) (map[string]any, error) {
	return func(ctx context.Context, input *UserMessage, opts ...any) (output map[string]any, err error) {
		modelContext := AssembleModelContext(ctx, input, time.Now())
		if !modelContext.UseKnowledge {
			emitRetrieval(ctx, agentOpts, RetrievalObservation{Triggered: false, Query: modelContext.KnowledgeQuery})
			return documentsOutput(nil), nil
		}

		r := agentOpts.Retriever
		if r == nil {
			r, err = newRetriever(ctx)
			if err != nil {
				g.Log().Warningf(ctx, "context knowledge retriever unavailable, err=%v", err)
				emitRetrieval(ctx, agentOpts, RetrievalObservation{Triggered: true, Query: modelContext.KnowledgeQuery, Error: err.Error()})
				return documentsOutput(nil), nil
			}
		}

		docs, err := r.Retrieve(ctx, modelContext.KnowledgeQuery)
		if err != nil {
			g.Log().Warningf(ctx, "context knowledge retrieval failed, err=%v", err)
			emitRetrieval(ctx, agentOpts, RetrievalObservation{Triggered: true, Query: modelContext.KnowledgeQuery, Error: err.Error()})
			return documentsOutput(nil), nil
		}

		finalDocs := docs
		stage := "retrieved"
		if !agentOpts.SkipRerank {
			reranked, err := rerankDocumentsLambda(ctx, map[string]any{
				"query":     modelContext.KnowledgeQuery,
				"documents": docs,
			})
			if err != nil {
				g.Log().Warningf(ctx, "context knowledge rerank failed, err=%v", err)
			} else {
				finalDocs = toDocuments(reranked["documents"])
				stage = "reranked"
			}
		}

		budgeted := budgetDocuments(finalDocs, agentOpts.documentsMax(), agentOpts.documentMax())
		emitRetrieval(ctx, agentOpts, RetrievalObservation{
			Triggered: true,
			Query:     modelContext.KnowledgeQuery,
			Docs:      documentEvidence(budgeted, stage),
		})
		return documentsOutput(budgeted), nil
	}
}

// newInputToChatLambda component initialization function of node 'InputToHistory' in graph 'EinoAgent'
func newInputToChatLambda(ctx context.Context, input *UserMessage, opts ...any) (output map[string]any, err error) {
	modelContext := AssembleModelContext(ctx, input, time.Now())
	modelContext.ProjectContext = project_context.Load(ctx)
	modelContext.Skills = skills.LoadSelectedContext(ctx, modelContext.Content)
	return modelContext.PromptValues(), nil
}

func documentsOutput(docs []*schema.Document) map[string]any {
	if docs == nil {
		docs = []*schema.Document{}
	}
	return map[string]any{"documents": docs}
}

package eval

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type fakeRetriever struct {
	docs []DocEvidence
}

func (r fakeRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	docs := make([]*schema.Document, 0, len(r.docs))
	for _, doc := range r.docs {
		meta := doc.Meta
		if meta == nil {
			meta = map[string]any{}
		}
		if doc.Source != "" {
			meta["_source"] = doc.Source
		}
		docs = append(docs, &schema.Document{Content: doc.Content, MetaData: meta})
	}
	return docs, nil
}

type fakeTool struct {
	name   string
	output string
	errMsg string
}

func (t fakeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: "eval fake tool " + t.name,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Desc: "query"},
		}),
	}, nil
}

func (t fakeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if t.errMsg != "" {
		return `{"success":false,"error":"` + escapeJSONString(t.errMsg) + `"}`, nil
	}
	if t.output != "" {
		return t.output, nil
	}
	return `{"success":true,"message":"fake tool executed"}`, nil
}

type fakeModel struct {
	mu        sync.Mutex
	caseData  Case
	toolNames []string
	calls     int
}

func (m *fakeModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.calls < len(m.toolNames) {
		toolName := m.toolNames[m.calls]
		m.calls++
		return schema.AssistantMessage("", []schema.ToolCall{
			{
				ID:   fmt.Sprintf("eval_tool_%d", m.calls),
				Type: "function",
				Function: schema.FunctionCall{
					Name:      toolName,
					Arguments: `{"query":"` + escapeJSONString(m.caseData.Query) + `"}`,
				},
			},
		}), nil
	}
	m.calls++
	return schema.AssistantMessage(mockAnswer(m.caseData), nil), nil
}

func (m *fakeModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *fakeModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func fakeTools(c Case) []tool.BaseTool {
	fixtures := map[string]ToolFixture{}
	for _, fixture := range c.MockContext.ToolOutputs {
		fixtures[fixture.Name] = fixture
	}

	names := c.Expected.Tools
	if len(names) == 0 {
		names = []string{"get_current_time", "query_prometheus_alerts", "query_internal_docs", "mysql_crud"}
	}
	tools := make([]tool.BaseTool, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		fixture := fixtures[name]
		tools = append(tools, fakeTool{name: name, output: fixture.Output, errMsg: fixture.Error})
	}
	return tools
}

func mockAnswer(c Case) string {
	if strings.TrimSpace(c.MockContext.MockAnswer) != "" {
		return c.MockContext.MockAnswer
	}
	parts := append([]string{}, c.Expected.AnswerContains...)
	if len(parts) == 0 {
		parts = append(parts, "ok")
	}
	return strings.Join(parts, " ")
}

func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

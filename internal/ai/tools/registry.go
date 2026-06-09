package tools

import (
	"SREagent/internal/authz"
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/gogf/gf/v2/frame/g"
)

type Scenario string

const (
	ScenarioChat  Scenario = "chat"
	ScenarioAIOps Scenario = "aiops"
)

type Registry struct{}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) ToolsFor(ctx context.Context, scenario Scenario) ([]tool.BaseTool, error) {
	switch scenario {
	case ScenarioChat:
		return r.chatTools(ctx)
	case ScenarioAIOps:
		return r.aiOpsTools(ctx)
	default:
		return nil, fmt.Errorf("unsupported tool scenario: %s", scenario)
	}
}

func (r *Registry) chatTools(ctx context.Context) ([]tool.BaseTool, error) {
	toolList, err := r.optionalLogMcpTools(ctx)
	if err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewPrometheusAlertsQueryTool); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewMysqlCrudTool); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewMysqlSlowSQLTool); err != nil {
		return nil, err
	}
	if err = appendPaymentAgentTools(ctx, &toolList); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewGetCurrentTimeTool); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewQueryInternalDocsTool); err != nil {
		return nil, err
	}
	return toolList, nil
}

func (r *Registry) aiOpsTools(ctx context.Context) ([]tool.BaseTool, error) {
	toolList, err := r.optionalLogMcpTools(ctx)
	if err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewPrometheusAlertsQueryTool); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewMysqlSlowSQLTool); err != nil {
		return nil, err
	}
	if err = appendPaymentAgentTools(ctx, &toolList); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewQueryInternalDocsTool); err != nil {
		return nil, err
	}
	if err = appendTool(ctx, &toolList, NewGetCurrentTimeTool); err != nil {
		return nil, err
	}
	return toolList, nil
}

func appendTool(ctx context.Context, toolList *[]tool.BaseTool, factory func() (tool.InvokableTool, error)) error {
	t, err := factory()
	if err != nil {
		return err
	}
	info, err := t.Info(ctx)
	if err != nil {
		return err
	}
	if !toolAllowedForRegistration(ctx, info.Name, "") {
		return nil
	}
	*toolList = append(*toolList, wrapAuthorizedTool(t))
	return nil
}

func appendPaymentAgentTools(ctx context.Context, toolList *[]tool.BaseTool) error {
	for _, factory := range []func() (tool.InvokableTool, error){
		NewQueryPaymentAnomalyTool,
		NewQueryPaymentMetricsTool,
		NewQueryPaymentOrderDetailTool,
		NewCheckPaymentConsistencyTool,
		NewSearchPaymentLogsTool,
		NewQueryPaymentStatusDistributionTool,
	} {
		if err := appendTool(ctx, toolList, factory); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) optionalLogMcpTools(ctx context.Context) ([]tool.BaseTool, error) {
	if !configBool(ctx, "tools.mcp_log.enabled", true) {
		g.Log().Info(ctx, "mcp log tools disabled by config")
		return []tool.BaseTool{}, nil
	}
	if !toolAllowedForRegistration(ctx, "mcp_log", authz.PermLogsRead) {
		g.Log().Info(ctx, "mcp log tools hidden by permission policy")
		return []tool.BaseTool{}, nil
	}

	toolList, err := GetLogMcpTool(ctx)
	if err == nil {
		return wrapMCPLogTools(toolList), nil
	}
	if configBool(ctx, "tools.mcp_log.required", false) {
		return nil, err
	}
	g.Log().Warningf(ctx, "mcp log tools unavailable, continue without mcp log tools, err=%v", err)
	return []tool.BaseTool{}, nil
}

func wrapMCPLogTools(toolList []tool.BaseTool) []tool.BaseTool {
	wrapped := make([]tool.BaseTool, 0, len(toolList))
	for _, candidate := range toolList {
		invokable, ok := candidate.(tool.InvokableTool)
		if !ok {
			continue
		}
		wrapped = append(wrapped, wrapAuthorizedToolWithPermission(invokable, authz.PermLogsRead))
	}
	return wrapped
}

func configBool(ctx context.Context, key string, fallback bool) bool {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return fallback
	}
	return v.Bool()
}

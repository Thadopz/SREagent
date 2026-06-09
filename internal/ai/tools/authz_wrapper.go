package tools

import (
	"SREagent/internal/authz"
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type authorizedTool struct {
	inner              tool.InvokableTool
	overridePermission authz.Permission
}

func wrapAuthorizedTool(inner tool.InvokableTool) tool.InvokableTool {
	return &authorizedTool{inner: inner}
}

func wrapAuthorizedToolWithPermission(inner tool.InvokableTool, permission authz.Permission) tool.InvokableTool {
	return &authorizedTool{inner: inner, overridePermission: permission}
}

func (t *authorizedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *authorizedTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	info, err := t.inner.Info(ctx)
	if err != nil {
		return "", err
	}
	call := buildToolCall(ctx, info.Name, argumentsInJSON, t.overridePermission)
	decision := authz.NewAuthorizer().Authorize(ctx, call)
	auditToolDecision(ctx, call, decision)
	if !decision.Allowed {
		return marshalToolError(nil, "tool authorization denied: "+decision.Reason), nil
	}
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

func toolAllowedForRegistration(ctx context.Context, toolName string, overridePermission authz.Permission) bool {
	call := buildToolCall(ctx, toolName, "", overridePermission)
	decision := authz.NewAuthorizer().Authorize(ctx, call)
	auditToolDecision(ctx, call, decision)
	return decision.Allowed
}

func buildToolCall(ctx context.Context, toolName string, argumentsInJSON string, overridePermission authz.Permission) authz.ToolCall {
	meta, _ := LookupMetadata(toolName)
	permission := meta.Permission
	if overridePermission != "" {
		permission = overridePermission
	}
	risk := meta.Risk
	requiresConfirm := meta.RequiresConfirm
	operation := ""

	if toolName == "mysql_crud" {
		operation = mysqlOperateTypeFromArgs(argumentsInJSON)
		if IsWriteOperation(operation) {
			permission = authz.PermMySQLWrite
			risk = authz.RiskWrite
			requiresConfirm = true
		} else {
			permission = authz.PermMySQLRead
			risk = authz.RiskReadOnly
			requiresConfirm = false
		}
	}

	return authz.ToolCall{
		Name:               toolName,
		Permission:         permission,
		Risk:               risk,
		RequiresConfirm:    requiresConfirm,
		Confirmed:          false,
		ConversationID:     contextString(ctx, "client_id"),
		ArgumentsInJSON:    argumentsInJSON,
		Operation:          operation,
		ServerWriteEnabled: mysqlWriteEnabled(ctx),
	}
}

func mysqlOperateTypeFromArgs(argumentsInJSON string) string {
	if strings.TrimSpace(argumentsInJSON) == "" {
		return "query"
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &payload); err != nil {
		return "query"
	}
	if raw, ok := payload["operate_type"].(string); ok {
		op := strings.ToLower(strings.TrimSpace(raw))
		if op != "" {
			return op
		}
	}
	return "query"
}

func auditToolDecision(ctx context.Context, call authz.ToolCall, decision authz.Decision) {
	principal := authz.PrincipalFromContext(ctx)
	g.Log().Infof(ctx,
		"[authz] user=%s tenant=%s roles=%s tool=%s permission=%s risk=%s operation=%s conversation_id=%s allowed=%t reason=%s",
		principal.UserID,
		principal.TenantID,
		strings.Join(principal.Roles, ","),
		call.Name,
		call.Permission,
		call.Risk,
		call.Operation,
		call.ConversationID,
		decision.Allowed,
		decision.Reason,
	)
}

func contextString(ctx context.Context, key string) string {
	if v, ok := ctx.Value(key).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

package tools

import (
	"context"
	"os"
	"strings"

	e_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

/*
	GetLogMcpTool

https://cloud.tencent.com/developer/mcp/server/11710
https://cloud.tencent.com/document/product/614/118699#90415b66-8edb-43a9-ad5a-c2b0a97f5eaf

https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/tool/tool_mcp/
https://mcp-go.dev/clients
*/
func GetLogMcpTool(ctx context.Context) ([]tool.BaseTool, error) {
	mcpUrl := getMCPLogURL(ctx)
	cli, err := client.NewSSEMCPClient(mcpUrl)
	if err != nil {
		return []tool.BaseTool{}, err
	}
	err = cli.Start(ctx)
	if err != nil {
		return []tool.BaseTool{}, err
	}
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}
	if _, err = cli.Initialize(ctx, initRequest); err != nil {
		return []tool.BaseTool{}, err
	}
	mcpTools, err := e_mcp.GetTools(ctx, &e_mcp.Config{Cli: cli})
	if err != nil {
		return []tool.BaseTool{}, err
	}
	return mcpTools, nil
}

func getMCPLogURL(ctx context.Context) string {
	for _, key := range []string{"tools.mcp_log.url", "mcp_url"} {
		v, err := g.Cfg().Get(ctx, key)
		if err != nil || v == nil {
			continue
		}
		url := strings.TrimSpace(v.String())
		if url != "" {
			return url
		}
	}
	if url := strings.TrimSpace(os.Getenv("SUPERBIZ_MCP_LOG_URL")); url != "" {
		return url
	}
	return "https://mcp-api.tencent-cloud.com/sse/YOUR_MCP_INSTANCE_ID"
}

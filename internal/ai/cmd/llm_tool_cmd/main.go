package main

import (
	tools2 "SREagent/internal/ai/tools"
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 创建 ChatModel
	config := &openai.ChatModelConfig{
		APIKey:  os.Getenv("SREAGENT_CHAT_API_KEY"),
		Model:   os.Getenv("SREAGENT_CHAT_MODEL"),
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
	}
	chatModel, err := openai.NewChatModel(ctx, config)
	if err != nil {
		panic(err)
	}
	// 获取工具信息, 用于绑定到 ChatModel
	toolList, err := tools2.GetLogMcpTool(ctx)
	if err != nil {
		panic(fmt.Errorf("load MCP tools failed: %w", err))
	}
	currentTimeTool, err := tools2.NewGetCurrentTimeTool()
	if err != nil {
		panic(fmt.Errorf("load current time tool failed: %w", err))
	}
	toolList = append(toolList, currentTimeTool)
	fmt.Printf("tools loaded: %d\n", len(toolList))
	toolInfos := make([]*schema.ToolInfo, 0)
	var info *schema.ToolInfo
	for _, todoTool := range toolList {
		info, err = todoTool.Info(ctx)
		if err != nil {
			panic(err)
		}
		fmt.Printf("tool name: %s\n", info.Name)
		toolInfos = append(toolInfos, info)
	}

	// 将 tools 绑定到 ChatModel
	err = chatModel.BindTools(toolInfos)
	if err != nil {
		panic(err)
	}

	// 创建一个完整的处理链
	chain := compose.NewChain[[]*schema.Message, *schema.Message]()
	chain.AppendChatModel(chatModel, compose.WithNodeName("chat_model"))

	// 编译并运行 chain
	agent, err := chain.Compile(ctx)
	if err != nil {
		panic(err)
	}
	// 运行示例
	resp, err := agent.Invoke(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "告诉我你有哪些工具可以使用",
		},
	})
	if err != nil {
		panic(err)
	}
	// 输出结果
	fmt.Println(resp.Content)
}

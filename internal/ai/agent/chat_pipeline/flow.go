package chat_pipeline

import (
	"SREagent/internal/ai/tools"
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	return newReactAgentLambdaWithOptions(ctx, ChatAgentOptions{})
}

func newReactAgentLambdaWithOptions(ctx context.Context, opts ChatAgentOptions) (lba *compose.Lambda, err error) {
	maxStep := opts.ReactMaxStep
	if maxStep <= 0 {
		maxStep = 25
	}
	config := &react.AgentConfig{
		MaxStep:            maxStep,
		ToolReturnDirectly: map[string]struct{}{}}
	chatModelIns11 := opts.ChatModel
	if chatModelIns11 == nil {
		chatModelIns11, err = newChatModel(ctx)
		if err != nil {
			return nil, err
		}
	}
	config.ToolCallingModel = chatModelIns11
	//searchTool, err := newSearchTool(ctx)
	//if err != nil {
	//	return nil, err
	//}
	toolList := opts.Tools
	if toolList == nil {
		toolList, err = tools.NewRegistry().ToolsFor(ctx, tools.ScenarioChat)
		if err != nil {
			return nil, err
		}
	}
	config.ToolsConfig.Tools = toolList

	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	return lba, nil
}

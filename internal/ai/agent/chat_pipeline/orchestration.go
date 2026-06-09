package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func BuildChatAgent(ctx context.Context) (r compose.Runnable[*UserMessage, *schema.Message], err error) {
	return BuildChatAgentWithOptions(ctx, ChatAgentOptions{})
}

func BuildChatAgentWithOptions(ctx context.Context, opts ChatAgentOptions) (r compose.Runnable[*UserMessage, *schema.Message], err error) {
	const (
		ContextDocuments = "ContextDocuments"
		ChatTemplate     = "ChatTemplate"
		ReactAgent       = "ReactAgent"
		InputToChat      = "InputToChat"
	)

	g := compose.NewGraph[*UserMessage, *schema.Message]()
	_ = g.AddLambdaNode(ContextDocuments, compose.InvokableLambdaWithOption(newContextDocumentsLambdaWithOptions(opts)), compose.WithNodeName("ContextDocuments"))

	chatTemplateKeyOfChatTemplate, err := newChatTemplate(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatTemplateNode(ChatTemplate, chatTemplateKeyOfChatTemplate)

	reactAgentKeyOfLambda, err := newReactAgentLambdaWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode(ReactAgent, reactAgentKeyOfLambda, compose.WithNodeName("ReActAgent"))
	_ = g.AddLambdaNode(InputToChat, compose.InvokableLambdaWithOption(newInputToChatLambda), compose.WithNodeName("UserMessageToChat"))

	_ = g.AddEdge(compose.START, ContextDocuments)
	_ = g.AddEdge(compose.START, InputToChat)
	_ = g.AddEdge(ContextDocuments, ChatTemplate)
	_ = g.AddEdge(InputToChat, ChatTemplate)
	_ = g.AddEdge(ChatTemplate, ReactAgent)
	_ = g.AddEdge(ReactAgent, compose.END)

	r, err = g.Compile(ctx, compose.WithGraphName("ChatAgent"), compose.WithNodeTriggerMode(compose.AllPredecessor))
	if err != nil {
		return nil, err
	}
	return r, nil
}

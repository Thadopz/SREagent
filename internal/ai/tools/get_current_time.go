package tools

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type GetCurrentTimeInput struct{}

type GetCurrentTimeOutput struct {
	Success      bool   `json:"success" jsonschema:"description=Indicates whether the time retrieval was successful"`
	Seconds      int64  `json:"seconds" jsonschema:"description=Current Unix timestamp in seconds since epoch"`
	Milliseconds int64  `json:"milliseconds" jsonschema:"description=Current Unix timestamp in milliseconds since epoch"`
	Microseconds int64  `json:"microseconds" jsonschema:"description=Current Unix timestamp in microseconds since epoch"`
	Timestamp    string `json:"timestamp" jsonschema:"description=Human-readable timestamp"`
	Message      string `json:"message" jsonschema:"description=Status message"`
}

func NewGetCurrentTimeTool() (tool.InvokableTool, error) {
	t, err := utils.InferOptionableTool(
		"get_current_time",
		"Get current system time in multiple formats. Returns current time in seconds, milliseconds, microseconds, and a human-readable timestamp.",
		func(ctx context.Context, input *GetCurrentTimeInput, opts ...tool.Option) (string, error) {
			now := time.Now()
			timeOutput := GetCurrentTimeOutput{
				Success:      true,
				Seconds:      now.Unix(),
				Milliseconds: now.UnixMilli(),
				Microseconds: now.UnixMicro(),
				Timestamp:    now.Format("2006-01-02 15:04:05.000000"),
				Message:      "Current time retrieved successfully",
			}

			log.Printf("Current time: Seconds=%d, Milliseconds=%d, Microseconds=%d", timeOutput.Seconds, timeOutput.Milliseconds, timeOutput.Microseconds)
			return marshalToolData(timeOutput, "current time retrieved successfully"), nil
		})
	if err != nil {
		return nil, err
	}

	return t, nil
}

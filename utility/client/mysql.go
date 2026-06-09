package client

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

func InitMySQL(ctx context.Context) error {
	if err := g.DB().PingMaster(); err != nil {
		return fmt.Errorf("ping goframe mysql default group failed: %w", err)
	}
	return nil
}

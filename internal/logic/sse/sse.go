package sse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// Client represents one SSE connection.
type Client struct {
	Id          string
	Request     *ghttp.Request
	messageChan chan string
}

// Service manages SSE connections.
type Service struct {
	clients *gmap.StrAnyMap
}

func New() *Service {
	return &Service{
		clients: gmap.NewStrAnyMap(true),
	}
}

func (s *Service) Create(ctx context.Context, r *ghttp.Request) (*Client, error) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")

	clientId := r.Get("client_id", guid.S()).String()
	client := &Client{
		Id:          clientId,
		Request:     r,
		messageChan: make(chan string, 100),
	}
	client.SendToClient("connected", fmt.Sprintf(`{"status":"connected","client_id":"%s"}`, clientId))
	return client, nil
}

func (c *Client) SendToClient(eventType, data string) bool {
	msg := formatEvent(eventType, data)
	c.Request.Response.Writefln("%s", msg)
	c.Request.Response.Flush()
	return true
}

func formatEvent(eventType, data string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("id: %d\n", time.Now().UnixNano()))
	b.WriteString("event: ")
	b.WriteString(sanitizeEventType(eventType))
	b.WriteString("\n")

	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	for _, line := range strings.Split(data, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func sanitizeEventType(eventType string) string {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return "message"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-':
			return r
		default:
			return -1
		}
	}, eventType)
}

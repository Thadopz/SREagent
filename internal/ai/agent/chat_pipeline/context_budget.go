package chat_pipeline

import (
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	MaxPromptContextChars = 24000
	HistoryMaxChars       = 8000
	DocumentsMaxChars     = 8000
	SummaryMaxChars       = 3000
	SessionStateMaxChars  = 1500
	DurableMemoryMaxChars = 2000
	ToolResultsMaxChars   = 2500
	DocumentMaxChars      = 2500
)

func trimRunesHead(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[len(runes)-max:])
}

func trimRunesTail(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func budgetHistoryMessages(history []*schema.Message, maxChars int) []*schema.Message {
	if len(history) == 0 {
		return nil
	}
	if maxChars <= 0 {
		return []*schema.Message{}
	}

	cloned := cloneHistory(history)
	total := 0
	start := len(cloned)
	for i := len(cloned) - 1; i >= 0; i-- {
		if cloned[i] == nil {
			continue
		}
		nextTotal := total + utf8.RuneCountInString(cloned[i].Content)
		if nextTotal > maxChars && start < len(cloned) {
			break
		}
		total = nextTotal
		start = i
	}
	if start >= len(cloned) {
		return []*schema.Message{}
	}

	start = alignHistoryPairStart(cloned, start)
	if start >= len(cloned) {
		return []*schema.Message{}
	}

	budgeted := cloneHistory(cloned[start:])
	total = 0
	for _, msg := range budgeted {
		if msg == nil {
			continue
		}
		total += utf8.RuneCountInString(msg.Content)
	}
	if total > maxChars && len(budgeted) > 0 {
		excess := total - maxChars
		first := budgeted[0]
		firstLen := utf8.RuneCountInString(first.Content)
		if firstLen > excess {
			first.Content = trimRunesHead(first.Content, firstLen-excess)
		}
	}
	return budgeted
}

func alignHistoryPairStart(messages []*schema.Message, start int) int {
	if start <= 0 || start >= len(messages) {
		return start
	}
	if isUserMessage(messages[start]) {
		return start
	}
	if isUserMessage(messages[start-1]) {
		return start - 1
	}
	for start < len(messages) && !isUserMessage(messages[start]) {
		start++
	}
	return start
}

func isUserMessage(msg *schema.Message) bool {
	return msg != nil && string(msg.Role) == "user"
}

func budgetDocuments(docs []*schema.Document, maxChars int, perDocMaxChars int) []*schema.Document {
	if len(docs) == 0 || maxChars <= 0 || perDocMaxChars <= 0 {
		return []*schema.Document{}
	}

	budgeted := make([]*schema.Document, 0, len(docs))
	total := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		remaining := maxChars - total
		if remaining <= 0 {
			break
		}

		docCopy := *doc
		limit := perDocMaxChars
		if remaining < limit {
			limit = remaining
		}
		docCopy.Content = trimRunesTail(docCopy.Content, limit)
		if docCopy.Content == "" {
			continue
		}
		total += utf8.RuneCountInString(docCopy.Content)
		budgeted = append(budgeted, &docCopy)
	}
	return budgeted
}

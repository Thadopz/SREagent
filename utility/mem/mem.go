package mem

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	defaultStoredWindowSize  = 20
	defaultContextMaxMessage = 8
	defaultContextMaxChars   = 12000
	defaultSummaryMaxChars   = 4000
)

var SimpleMemoryMap = make(map[string]*SimpleMemory)
var mu sync.Mutex
var store memoryStore = newMySQLMemoryStore()

type ContextPolicy struct {
	MaxMessages int
	MaxChars    int
}

type memoryScope struct {
	UserID         string
	ConversationID string
	StorageID      string
}

func DefaultContextPolicy() ContextPolicy {
	return ContextPolicy{
		MaxMessages: defaultContextMaxMessage,
		MaxChars:    defaultContextMaxChars,
	}
}

func GetSimpleMemory(id string) *SimpleMemory {
	return GetConversationMemory(id, id)
}

func GetConversationMemory(userID string, conversationID string) *SimpleMemory {
	mu.Lock()
	defer mu.Unlock()

	scope := normalizeMemoryScope(userID, conversationID)
	if mem, ok := SimpleMemoryMap[scope.StorageID]; ok {
		if mem.UserID == "" {
			mem.UserID = scope.UserID
		}
		return mem
	}

	newMem := newSimpleMemory(scope.UserID, scope.StorageID, store)
	SimpleMemoryMap[scope.StorageID] = newMem
	return newMem
}

type SimpleMemory struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id"`
	Messages      []*schema.Message `json:"messages"`
	Summary       string            `json:"summary"`
	MaxWindowSize int
	UpdatedAt     time.Time
	mu            sync.Mutex
	store         memoryStore
}

func (c *SimpleMemory) SetMessages(msg *schema.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	c.trimStoredMessages()
	c.persistLocked(msg)
}

func (c *SimpleMemory) GetMessages() []*schema.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloneMessages(c.Messages)
}

func (c *SimpleMemory) GetSummary() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Summary
}

func (c *SimpleMemory) GetContextMessages(policy ContextPolicy) []*schema.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	maxMessages := normalizeMaxMessages(policy.MaxMessages)
	maxChars := policy.MaxChars
	if maxChars <= 0 {
		maxChars = defaultContextMaxChars
	}

	start := 0
	if len(c.Messages) > maxMessages {
		start = len(c.Messages) - maxMessages
	}
	start = alignPairStart(start, len(c.Messages))

	totalChars := 0
	for i := len(c.Messages) - 1; i >= start; i-- {
		if c.Messages[i] == nil {
			continue
		}
		nextTotal := totalChars + utf8.RuneCountInString(c.Messages[i].Content)
		if nextTotal > maxChars && i < len(c.Messages)-1 {
			start = i + 1
			break
		}
		totalChars = nextTotal
	}
	start = alignPairStart(start, len(c.Messages))
	if start >= len(c.Messages) {
		return []*schema.Message{}
	}

	return cloneMessages(c.Messages[start:])
}

func (c *SimpleMemory) trimStoredMessages() {
	maxWindowSize := normalizeMaxMessages(c.MaxWindowSize)
	if len(c.Messages) <= maxWindowSize {
		return
	}

	excess := len(c.Messages) - maxWindowSize
	if excess%2 != 0 {
		excess++
	}
	if excess >= len(c.Messages) {
		c.mergeSummary(c.Messages)
		c.Messages = []*schema.Message{}
		return
	}

	c.mergeSummary(c.Messages[:excess])
	c.Messages = c.Messages[excess:]
}

func (c *SimpleMemory) mergeSummary(messages []*schema.Message) {
	if len(messages) == 0 {
		return
	}

	summaryPart := summarizeMessages(messages)
	if summaryPart == "" {
		return
	}

	if strings.TrimSpace(c.Summary) == "" {
		c.Summary = summaryPart
	} else {
		c.Summary = c.Summary + "\n" + summaryPart
	}
	c.Summary = trimRunes(c.Summary, defaultSummaryMaxChars)
}

func normalizeMaxMessages(maxMessages int) int {
	if maxMessages <= 0 {
		maxMessages = defaultContextMaxMessage
	}
	if maxMessages%2 != 0 {
		maxMessages--
	}
	if maxMessages <= 0 {
		return 2
	}
	return maxMessages
}

func alignPairStart(start int, total int) int {
	if start%2 != 0 && start < total {
		start++
	}
	return start
}

func cloneMessages(messages []*schema.Message) []*schema.Message {
	cloned := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		msgCopy := *msg
		cloned = append(cloned, &msgCopy)
	}
	return cloned
}

func summarizeMessages(messages []*schema.Message) string {
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(strings.ReplaceAll(msg.Content, "\n", " "))
		if content == "" {
			continue
		}
		lines = append(lines, string(msg.Role)+": "+trimRunes(content, 300))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func trimRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[len(runes)-max:])
}

func newSimpleMemory(userID string, id string, store memoryStore) *SimpleMemory {
	mem := &SimpleMemory{
		ID:            id,
		UserID:        userID,
		Messages:      []*schema.Message{},
		MaxWindowSize: defaultStoredWindowSize,
		UpdatedAt:     time.Now(),
		store:         store,
	}
	if store == nil {
		return mem
	}

	snapshot, err := store.Load(id)
	if err != nil || snapshot == nil {
		return mem
	}

	mem.Messages = cloneMessages(snapshot.Messages)
	mem.UserID = normalizeScopeID(snapshot.UserID, userID)
	mem.Summary = snapshot.Summary
	mem.UpdatedAt = snapshot.UpdatedAt
	if snapshot.MaxWindowSize > 0 {
		mem.MaxWindowSize = snapshot.MaxWindowSize
	}
	mem.trimStoredMessages()
	return mem
}

func (c *SimpleMemory) persistLocked(msg *schema.Message) {
	if c.store == nil {
		return
	}
	snapshot := &memorySnapshot{
		ID:            c.ID,
		UserID:        c.UserID,
		Messages:      cloneMessages(c.Messages),
		Summary:       c.Summary,
		MaxWindowSize: c.MaxWindowSize,
		UpdatedAt:     c.UpdatedAt,
	}
	if appender, ok := c.store.(appendMemoryStore); ok && msg != nil {
		_ = appender.Append(snapshot, msg)
		return
	}
	_ = c.store.Save(snapshot)
}

func normalizeScopeID(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func normalizeMemoryScope(userID string, conversationID string) memoryScope {
	userID = strings.TrimSpace(userID)
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = userID
	}
	if userID == "" {
		userID = conversationID
	}
	if userID == "" {
		userID = "anonymous"
	}
	if conversationID == "" {
		conversationID = userID
	}
	return memoryScope{
		UserID:         userID,
		ConversationID: conversationID,
		StorageID:      scopedConversationStorageID(userID, conversationID),
	}
}

func scopedConversationStorageID(userID string, conversationID string) string {
	userID = strings.TrimSpace(userID)
	conversationID = strings.TrimSpace(conversationID)
	if userID == "" || userID == conversationID {
		return conversationID
	}
	sum := sha256.Sum256([]byte(userID + "\x00" + conversationID))
	return "scoped_" + hex.EncodeToString(sum[:])[:32]
}

func setMemoryStoreForTest(testStore memoryStore) func() {
	mu.Lock()
	previousStore := store
	previousMemoryMap := SimpleMemoryMap
	store = testStore
	SimpleMemoryMap = make(map[string]*SimpleMemory)
	mu.Unlock()

	return func() {
		mu.Lock()
		store = previousStore
		SimpleMemoryMap = previousMemoryMap
		mu.Unlock()
	}
}

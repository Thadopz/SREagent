package chat

import "testing"

func TestResolveMemoryScope(t *testing.T) {
	userID, conversationID := resolveMemoryScope("legacy", "", "")
	if userID != "legacy" || conversationID != "legacy" {
		t.Fatalf("expected legacy id fallback, got user=%q conversation=%q", userID, conversationID)
	}

	userID, conversationID = resolveMemoryScope("legacy", "user-1", "conversation-1")
	if userID != "user-1" || conversationID != "conversation-1" {
		t.Fatalf("expected explicit scope, got user=%q conversation=%q", userID, conversationID)
	}
}

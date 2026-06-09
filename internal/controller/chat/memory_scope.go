package chat

import "strings"

func resolveMemoryScope(id string, userID string, conversationID string) (string, string) {
	id = strings.TrimSpace(id)
	userID = strings.TrimSpace(userID)
	conversationID = strings.TrimSpace(conversationID)

	if conversationID == "" {
		conversationID = id
	}
	if userID == "" {
		userID = id
	}
	if userID == "" {
		userID = conversationID
	}
	if conversationID == "" {
		conversationID = userID
	}
	return userID, conversationID
}

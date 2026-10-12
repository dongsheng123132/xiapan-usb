package main

import (
	"encoding/json"
	"strings"
)

type offlineReference struct {
	ID      string     `json:"id"`
	Match   [][]string `json:"match"`
	Chinese string     `json:"zh"`
	English string     `json:"en"`
}

// These are disclosed reference cards, never presented as model generations.
// The first matching card wins; each match group must contain a matching term.
func offlineReferenceReply(s *ChatSession, message, language string) string {
	if !s.OfflineOnly {
		return ""
	}
	b, err := assets.ReadFile("catalog/offline-references.json")
	if err != nil {
		return ""
	}
	var cards []offlineReference
	if json.Unmarshal(b, &cards) != nil {
		return ""
	}
	context := strings.ToLower(message)
	// Preserve a physical-disk warning for short follow-ups such as "can I scan?".
	if len([]rune(message)) < 40 {
		skippedCurrent := false
		for i := len(s.Messages) - 1; i >= 0; i-- {
			m := s.Messages[i]
			if m.Role != "user" {
				continue
			}
			if !skippedCurrent && m.Text == message {
				skippedCurrent = true
				continue
			}
			context += "\n" + strings.ToLower(m.Text)
			break
		}
	}
	for _, card := range cards {
		matched := len(card.Match) > 0
		for _, group := range card.Match {
			found := false
			for _, term := range group {
				if strings.Contains(context, strings.ToLower(term)) {
					found = true
					break
				}
			}
			matched = matched && found
		}
		if matched {
			if language == "en" {
				return "Maintenance reference (no model was called)\n\n" + card.English
			}
			return "维护知识指引（未调用模型）\n\n" + card.Chinese
		}
	}
	return ""
}

package channels

import (
	"encoding/json"
	"testing"
)

// TestParseSubscriptions проверяет разбор JSON-вывода yt-dlp:
// извлечение channel_id, дедупликация и отбрасывание некорректных записей.
func TestParseSubscriptions(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"entries": []map[string]any{
			{"id": "UC11111111111111111111", "channel_id": "UC11111111111111111111", "title": "Reef Tank"},
			{"id": "UC22222222222222222222", "channel_id": "UC22222222222222222222", "title": "Trading Daily"},
			{"id": "UC33333333333333333333", "channel_id": "UC33333333333333333333", "title": "Duplicate"},
			{"id": "UC33333333333333333333", "channel_id": "UC33333333333333333333", "title": "Duplicate"},
			{"id": "not-a-channel", "channel_id": "not-a-channel", "title": "Invalid"},
			{"id": "", "channel_id": "", "title": "Empty"},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	subs, err := parseSubscriptions(raw)
	if err != nil {
		t.Fatalf("parseSubscriptions: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("expected 3 unique subscriptions, got %d", len(subs))
	}

	seen := map[string]bool{}
	for _, s := range subs {
		if seen[s.ChannelID] {
			t.Errorf("duplicate channel_id %q", s.ChannelID)
		}
		seen[s.ChannelID] = true
		if !validChannelID(s.ChannelID) {
			t.Errorf("invalid channel_id %q passed filter", s.ChannelID)
		}
		if s.URL == "" {
			t.Errorf("empty url for channel %q", s.ChannelID)
		}
	}
}

// TestParseSubscriptionsInvalidJSON проверяет обработку мусора на входе.
func TestParseSubscriptionsInvalidJSON(t *testing.T) {
	if _, err := parseSubscriptions([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

// TestValidChannelID проверяет формат идентификатора канала.
func TestValidChannelID(t *testing.T) {
	cases := map[string]bool{
		"UCabcdefghijklmnopqrstu": true,
		"UCshort":                 false,
		"abcdefghijklmnopqrstu":   false,
		"":                        false,
	}
	for id, want := range cases {
		if got := validChannelID(id); got != want {
			t.Errorf("validChannelID(%q) = %v, want %v", id, got, want)
		}
	}
}

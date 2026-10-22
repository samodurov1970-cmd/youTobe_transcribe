package channels

import "testing"

// TestClassifyChannel проверяет, что каналы попадают строго в одну из трёх тем.
func TestClassifyChannel(t *testing.T) {
	cases := []struct {
		name        string
		channelName string
		description string
		wantTopic   string
		wantIsNews  bool
	}{
		{
			name:        "news by keyword",
			channelName: "BBC News",
			description: "latest headlines",
			wantTopic:   TopicNews,
			wantIsNews:  true,
		},
		{
			name:        "aquarium by keyword",
			channelName: "My Marine Aquarium",
			description: "reef and coral care",
			wantTopic:   TopicAquarium,
			wantIsNews:  false,
		},
		{
			name:        "trading by keyword",
			channelName: "Trading Gold Daily",
			description: "investing and forex",
			wantTopic:   TopicTrading,
			wantIsNews:  false,
		},
		{
			name:        "technical channel falls back to news",
			channelName: "Backend Development Podcast",
			description: "programming and coding",
			wantTopic:   TopicNews,
			wantIsNews:  true,
		},
		{
			name:        "unknown channel falls back to news",
			channelName: "Some Random Channel",
			description: "nothing relevant",
			wantTopic:   TopicNews,
			wantIsNews:  true,
		},
		{
			name:        "russian trading channel is not news",
			channelName: "Чингиз Шарипов",
			description: "ТГ канал про Трейдинг",
			wantTopic:   TopicTrading,
			wantIsNews:  false,
		},
		{
			name:        "russian investing channel is not news",
			channelName: "Фондовый рынок",
			description: "Инвестиции в акции и облигации",
			wantTopic:   TopicTrading,
			wantIsNews:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			topic, isNews := classifyChannel(tc.channelName, tc.description)
			if topic != tc.wantTopic {
				t.Errorf("topic = %q, want %q", topic, tc.wantTopic)
			}
			if isNews != tc.wantIsNews {
				t.Errorf("isNews = %v, want %v", isNews, tc.wantIsNews)
			}
			if !isAllowedTopic(topic) {
				t.Errorf("topic %q is not one of the allowed topics", topic)
			}
		})
	}
}

// TestClassifyChannelAlwaysAllowedTopic проверяет, что классификация
// никогда не возвращает тему вне списка разрешённых.
func TestClassifyChannelAlwaysAllowedTopic(t *testing.T) {
	names := []string{
		"", "Reef", "Trading", "News", "crypto", "ии", "aquarium gold",
	}
	for _, n := range names {
		topic, _ := classifyChannel(n, n)
		if !isAllowedTopic(topic) {
			t.Errorf("classifyChannel(%q) returned disallowed topic %q", n, topic)
		}
	}
}

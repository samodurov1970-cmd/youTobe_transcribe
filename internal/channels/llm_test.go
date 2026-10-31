package channels

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		prompt := buildPrompt("My Channel", "Some description")
		if !strings.Contains(prompt, "Channel name: My Channel") {
			t.Errorf("prompt should contain channel name")
		}
		if !strings.Contains(prompt, "Description: Some description") {
			t.Errorf("prompt should contain description")
		}
		if !strings.Contains(prompt, "Answer with one digit (1, 2, or 3)") {
			t.Errorf("prompt should contain classification instruction")
		}
	})

	t.Run("empty_description", func(t *testing.T) {
		prompt := buildPrompt("Test Channel", "")
		if strings.Contains(prompt, "Description:") {
			t.Errorf("prompt should not contain description section when empty")
		}
	})
}

func TestParseTopic(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected string
	}{
		{"digit_1", "1", TopicNews},
		{"digit_2", "2", TopicAquarium},
		{"digit_3", "3", TopicTrading},
		{"digit_1_with_whitespace", "  1\n\n", TopicNews},
		{"digit_2_with_whitespace", "\n 2 \n", TopicAquarium},
		{"digit_3_with_whitespace", " 3", TopicTrading},
		{"empty", "", ""},
		{"unrecognized", "4", ""},
		{"unrecognized_letter", "a", ""},
		{"text_response", "news channel", ""},
		{"multiline_1", "some text\n1\nmore text", TopicNews},
		{"multiline_2", "some text\n2\nmore text", TopicAquarium},
		{"multiline_3", "some text\n3\nmore text", TopicTrading},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTopic(tt.response)
			if result != tt.expected {
				t.Errorf("parseTopic(%q) = %q, expected %q", tt.response, result, tt.expected)
			}
		})
	}
}

func TestIsAllowedTopic(t *testing.T) {
	tests := []struct {
		topic    string
		expected bool
	}{
		{TopicNews, true},
		{TopicAquarium, true},
		{TopicTrading, true},
		{"invalid", false},
		{"", false},
		{"news", false},
		{"my_marine_aquarium", false},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			result := isAllowedTopic(tt.topic)
			if result != tt.expected {
				t.Errorf("isAllowedTopic(%q) = %v, expected %v", tt.topic, result, tt.expected)
			}
		})
	}
}
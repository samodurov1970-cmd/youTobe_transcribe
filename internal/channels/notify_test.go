package channels

import (
	"reflect"
	"testing"
)

// TestNewSubscriptionIDs проверяет выбор каналов, которые считаются новыми:
// ранее не встречавшиеся и ранее отписанные.
func TestNewSubscriptionIDs(t *testing.T) {
	known := map[string]bool{
		"UC11111111111111111111": true,  // подписан — не новый
		"UC22222222222222222222": true,  // подписан — не новый
		"UC33333333333333333333": false, // отписан — новый (повторная подписка)
	}
	subs := []subscription{
		{ChannelID: "UC11111111111111111111"},
		{ChannelID: "UC22222222222222222222"},
		{ChannelID: "UC33333333333333333333"},
		{ChannelID: "UC44444444444444444444"}, // ранее не встречался — новый
	}

	got := newSubscriptionIDs(known, subs)
	want := []string{"UC33333333333333333333", "UC44444444444444444444"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestNewSubscriptionIDsEmpty проверяет случай, когда новых подписок нет.
func TestNewSubscriptionIDsEmpty(t *testing.T) {
	known := map[string]bool{"UC11111111111111111111": true}
	subs := []subscription{{ChannelID: "UC11111111111111111111"}}

	if got := newSubscriptionIDs(known, subs); len(got) != 0 {
		t.Errorf("expected no new subscriptions, got %v", got)
	}
}

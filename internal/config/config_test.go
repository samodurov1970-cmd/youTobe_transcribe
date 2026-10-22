package config

import (
	"testing"
	"time"
)

// TestGetEnvDuration проверяет разбор интервала из переменной окружения:
// формат Go-длительности, число секунд и запасное значение при ошибке.
func TestGetEnvDuration(t *testing.T) {
	const key = "TEST_CHANNELS_INTERVAL"

	t.Setenv(key, "1m")
	if got := getEnvDuration(key, 24*time.Hour); got != time.Minute {
		t.Errorf("got %v, want 1m", got)
	}

	t.Setenv(key, "90")
	if got := getEnvDuration(key, 24*time.Hour); got != 90*time.Second {
		t.Errorf("got %v, want 90s", got)
	}

	t.Setenv(key, "not-a-duration")
	if got := getEnvDuration(key, 24*time.Hour); got != 24*time.Hour {
		t.Errorf("got %v, want default 24h", got)
	}

	t.Setenv(key, "")
	if got := getEnvDuration(key, 24*time.Hour); got != 24*time.Hour {
		t.Errorf("got %v, want default 24h", got)
	}
}

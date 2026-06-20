package service

import (
	"testing"
	"time"
)

func waitNotificationAsync(t *testing.T) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
}

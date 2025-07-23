package session_test

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/internal/session"
)

func TestSessionContextTTL(t *testing.T) {
	s := session.NewContextStore()
	s.Set("a", 1, true)
	s.WithExpiration("a", int64(1*time.Millisecond))
	time.Sleep(5 * time.Millisecond)
	if _, ok := s.Get("a"); ok {
		t.Error("Expired key still present")
	}
}

package commands

import (
	"testing"
	"time"
)

func TestParseTimeInput(t *testing.T) {
	now := time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC)

	t.Run("now", func(t *testing.T) {
		got, err := parseTimeInput("now", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(now) {
			t.Fatalf("expected %v, got %v", now, got)
		}
	})

	t.Run("relative", func(t *testing.T) {
		got, err := parseTimeInput("-15m", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := now.Add(-15 * time.Minute)
		if !got.Equal(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("rfc3339", func(t *testing.T) {
		in := "2026-02-25T07:30:00Z"
		got, err := parseTimeInput(in, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Date(2026, 2, 25, 7, 30, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})
}

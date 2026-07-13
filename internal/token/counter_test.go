package token

import (
	"strings"
	"testing"
)

func TestCounter_Record(t *testing.T) {
	c := &Counter{}

	// First observation
	line := c.Record(1200, 300)
	expected := "[tokens: 1,200 in / 300 out / 1,500 total | session: 1,200 in / 300 out / 1,500 total]"
	if line != expected {
		t.Errorf("expected: %q, got: %q", expected, line)
	}

	// Second observation
	line = c.Record(50, 150)
	expected = "[tokens: 50 in / 150 out / 200 total | session: 1,250 in / 450 out / 1,700 total]"
	if line != expected {
		t.Errorf("expected: %q, got: %q", expected, line)
	}
}

func TestCounter_Summary(t *testing.T) {
	c := &Counter{}
	c.Record(1000000, 5000)

	summary := c.Summary()
	if !strings.Contains(summary, "Session token usage:") {
		t.Errorf("expected summary to contain 'Session token usage:', got: %q", summary)
	}
	if !strings.Contains(summary, "1,000,000") {
		t.Errorf("expected summary to contain formatted input tokens '1,000,000', got: %q", summary)
	}
	if !strings.Contains(summary, "5,000") {
		t.Errorf("expected summary to contain formatted output tokens '5,000', got: %q", summary)
	}
	if !strings.Contains(summary, "1,005,000") {
		t.Errorf("expected summary to contain formatted total tokens '1,005,000', got: %q", summary)
	}
}

func TestFmtN64(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{9, "9"},
		{10, "10"},
		{999, "999"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{1234567890, "1,234,567,890"},
	}

	for _, tc := range tests {
		got := fmtN64(tc.input)
		if got != tc.expected {
			t.Errorf("fmtN64(%d) = %q, expected: %q", tc.input, got, tc.expected)
		}
	}
}

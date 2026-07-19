package memory

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestCompactReplacesOlderHistoryWithSummary(t *testing.T) {
	mem := NewMemory()
	mem.AddUserMessage("first request")
	mem.AddModelFunctionCall(&genai.FunctionCall{Name: "read_file", Args: map[string]any{"path": "main.go"}})
	mem.AddFunctionResponse("read_file", map[string]any{"content": "package main"})
	mem.AddUserMessage("latest request")

	if !mem.Compact(2) {
		t.Fatalf("expected compaction")
	}

	history := mem.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected summary plus 2 recent messages, got %d", len(history))
	}
	if history[0].Role != "user" || len(history[0].Parts) != 1 {
		t.Fatalf("expected summary user content, got %#v", history[0])
	}
	summary := history[0].Parts[0].Text
	if !strings.Contains(summary, "first request") {
		t.Fatalf("summary missing old user message: %s", summary)
	}
	if !strings.Contains(summary, "read_file") {
		t.Fatalf("summary missing tool call: %s", summary)
	}
	if history[2].Parts[0].Text != "latest request" {
		t.Fatalf("expected latest request to be retained verbatim")
	}
}

func TestCompactNoopsWhenHistoryFits(t *testing.T) {
	mem := NewMemory()
	mem.AddUserMessage("only request")

	if mem.Compact(2) {
		t.Fatalf("did not expect compaction")
	}
	if mem.Len() != 1 {
		t.Fatalf("expected history length 1, got %d", mem.Len())
	}
}

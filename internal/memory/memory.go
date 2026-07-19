package memory

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type Memory struct {
	history []*genai.Content
}

func NewMemory() *Memory {
	return &Memory{
		history: make([]*genai.Content, 0),
	}
}

func (m *Memory) AddUserMessage(text string) {
	m.history = append(m.history, &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: text},
		},
	})
}

func (m *Memory) AddModelMessage(text string) {
	m.history = append(m.history, &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{Text: text},
		},
	})
}

func (m *Memory) AddModelFunctionCall(call *genai.FunctionCall) {
	m.history = append(m.history, &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{FunctionCall: call},
		},
	})
}

func (m *Memory) AddFunctionResponse(name string, result map[string]any) {
	m.history = append(m.history, &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{FunctionResponse: &genai.FunctionResponse{
				Name:     name,
				Response: result,
			}},
		},
	})
}

func (m *Memory) AddModelContent(content *genai.Content) {
	m.history = append(m.history, content)
}

func (m *Memory) GetHistory() []*genai.Content {
	return m.history
}

func (m *Memory) Len() int {
	return len(m.history)
}

func (m *Memory) Compact(keepRecent int) bool {
	if keepRecent < 1 || len(m.history) <= keepRecent {
		return false
	}

	cutoff := len(m.history) - keepRecent
	summary := summarize(m.history[:cutoff])
	recent := append([]*genai.Content(nil), m.history[cutoff:]...)
	m.history = append([]*genai.Content{summaryContent(summary)}, recent...)
	return true
}

func summarize(history []*genai.Content) string {
	var lines []string
	lines = append(lines, "Conversation summary before compaction:")
	for _, content := range history {
		role := content.Role
		if role == "" {
			role = "unknown"
		}
		for _, part := range content.Parts {
			switch {
			case part.Text != "":
				lines = append(lines, fmt.Sprintf("- %s: %s", role, truncate(strings.TrimSpace(part.Text), 240)))
			case part.FunctionCall != nil:
				lines = append(lines, fmt.Sprintf("- %s called tool %s with %s", role, part.FunctionCall.Name, compactJSON(part.FunctionCall.Args)))
			case part.FunctionResponse != nil:
				lines = append(lines, fmt.Sprintf("- tool %s returned %s", part.FunctionResponse.Name, compactJSON(part.FunctionResponse.Response)))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func summaryContent(summary string) *genai.Content {
	return &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: summary},
		},
	}
}

func compactJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return truncate(fmt.Sprint(value), 200)
	}
	return truncate(string(data), 200)
}

func truncate(text string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

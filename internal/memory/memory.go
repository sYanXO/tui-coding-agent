package memory

import (
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

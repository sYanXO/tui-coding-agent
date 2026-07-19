package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"os"

	"google.golang.org/genai"
)

type ScriptResponse struct {
	Text      string       `json:"text,omitempty"`
	ToolCalls []ScriptCall `json:"tool_calls,omitempty"`
}

type ScriptCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type ScriptedProvider struct {
	responses []ScriptResponse
	next      int
}

func NewScriptedProvider(path string) (*ScriptedProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var responses []ScriptResponse
	if err := json.Unmarshal(data, &responses); err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, fmt.Errorf("script has no responses")
	}

	return &ScriptedProvider{responses: responses}, nil
}

func (p *ScriptedProvider) GenerateContentStream(ctx context.Context, history []*genai.Content, systemInstruction string, tools []*genai.Tool) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if p.next >= len(p.responses) {
			yield(responseFromText("script exhausted"), nil)
			return
		}

		scriptResp := p.responses[p.next]
		p.next++

		parts := make([]*genai.Part, 0, 1+len(scriptResp.ToolCalls))
		if scriptResp.Text != "" {
			parts = append(parts, &genai.Part{Text: scriptResp.Text})
		}
		for _, call := range scriptResp.ToolCalls {
			parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{
				Name: call.Name,
				Args: call.Args,
			}})
		}

		yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Role: "model", Parts: parts}},
			},
		}, nil)
	}
}

func responseFromText(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: text}},
				},
			},
		},
	}
}

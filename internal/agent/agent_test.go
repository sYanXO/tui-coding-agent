package agent

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/genai"

	"terminal-coding-agent/internal/executor"
	"terminal-coding-agent/internal/logger"
)

func init() {
	logger.Init()
}

type fakeProvider struct {
	responses []*genai.GenerateContentResponse
	calls     int
}

func (f *fakeProvider) GenerateContentStream(ctx context.Context, history []*genai.Content, systemInstruction string, tools []*genai.Tool) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		if f.calls >= len(f.responses) {
			return
		}
		resp := f.responses[f.calls]
		f.calls++
		yield(resp, nil)
	}
}

func responseWithFunctionCall(name string, args map[string]any) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{FunctionCall: &genai.FunctionCall{Name: name, Args: args}},
					},
				},
			},
		},
	}
}

func TestLoopStoresNativeFunctionCallsBeforeResponses(t *testing.T) {
	exec, err := executor.NewExecutor()
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	provider := &fakeProvider{
		responses: []*genai.GenerateContentResponse{
			responseWithFunctionCall("read_file", map[string]any{"path": "README.md"}),
			responseWithFunctionCall("finish", map[string]any{"message": "done"}),
		},
	}

	ag, err := NewAgentWithOptions(Options{
		Provider:     provider,
		ProviderName: "fake",
		Executor:     exec,
	})
	if err != nil {
		t.Fatalf("NewAgentWithOptions failed: %v", err)
	}

	if err := ag.HandleUserRequest(context.Background(), "read the readme"); err != nil {
		t.Fatalf("HandleUserRequest failed: %v", err)
	}

	history := ag.mem.GetHistory()
	if len(history) < 3 {
		t.Fatalf("expected at least user, model tool call, and function response entries, got %d", len(history))
	}

	modelToolCall := history[1]
	if modelToolCall.Role != "model" || len(modelToolCall.Parts) != 1 || modelToolCall.Parts[0].FunctionCall == nil {
		t.Fatalf("expected model function call at history[1], got %#v", modelToolCall)
	}
	if modelToolCall.Parts[0].FunctionCall.Name != "read_file" {
		t.Fatalf("expected read_file function call, got %q", modelToolCall.Parts[0].FunctionCall.Name)
	}

	toolResponse := history[2]
	if toolResponse.Role != "user" || len(toolResponse.Parts) != 1 || toolResponse.Parts[0].FunctionResponse == nil {
		t.Fatalf("expected function response at history[2], got %#v", toolResponse)
	}
}

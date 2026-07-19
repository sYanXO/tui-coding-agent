package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"

	"terminal-coding-agent/internal/executor"
	"terminal-coding-agent/internal/llm"
	mylogger "terminal-coding-agent/internal/logger"
	"terminal-coding-agent/internal/memory"
	"terminal-coding-agent/internal/prompt"
	"terminal-coding-agent/internal/token"
	"terminal-coding-agent/internal/tools"
)

type Agent struct {
	llmClient llm.Provider
	mem       *memory.Memory
	exec      *executor.Executor
	tools     []*genai.Tool
	tokens    *token.Counter
	Provider  string // "gemini" or "ollama"
}

func NewAgent(ctx context.Context, apiKey string) (*Agent, error) {
	var client llm.Provider
	var err error

	providerType := os.Getenv("LLM_PROVIDER")
	provider := "gemini"
	if providerType == "ollama" {
		client = llm.NewOllamaClient()
		provider = "ollama"
	} else {
		client, err = llm.NewGeminiClient(ctx, apiKey)
		if err != nil {
			return nil, err
		}
	}

	exec, err := executor.NewExecutor()
	if err != nil {
		return nil, err
	}

	return &Agent{
		llmClient: client,
		mem:       memory.NewMemory(),
		exec:      exec,
		tools:     tools.GetToolSchemas(),
		tokens:    &token.Counter{},
		Provider:  provider,
	}, nil
}

// HandleUserRequest runs the agent loop for the given input.
func (a *Agent) HandleUserRequest(ctx context.Context, input string) error {
	a.mem.AddUserMessage(input)
	return a.loop(ctx)
}

func (a *Agent) loop(ctx context.Context) error {
	maxIterations := 15
	iteration := 0

	for {
		if iteration >= maxIterations {
			msg := "Maximum iterations reached."
			mylogger.Error(msg)
			return fmt.Errorf("max iterations reached")
		}
		iteration++

		mylogger.AgentStream("Thinking... ")

		var fullText string
		var finalResponse *genai.GenerateContentResponse
		var nativeToolCalls []*genai.FunctionCall

		maxRetries := 3
		retryCount := 0

		for {
			iter := a.llmClient.GenerateContentStream(ctx, a.mem.GetHistory(), prompt.GetSystemInstruction(), a.tools)
			var streamErr error

			for resp, err := range iter {
				if err != nil {
					streamErr = err
					break
				}
				if resp == nil {
					continue
				}

				finalResponse = resp

				// Emit text chunks
				if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
					for _, part := range resp.Candidates[0].Content.Parts {
						if part.Text != "" {
							mylogger.AgentStream(part.Text)
							fullText += part.Text
						}
						if part.FunctionCall != nil {
							nativeToolCalls = append(nativeToolCalls, part.FunctionCall)
						}
					}
				}
			}

			if streamErr != nil {
				if strings.Contains(streamErr.Error(), "429") && retryCount < maxRetries {
					retryCount++
					retryMsg := fmt.Sprintf("Rate limit hit (429), retrying in 35 seconds (attempt %d/%d)...", retryCount, maxRetries)
					mylogger.System("\n" + retryMsg)
					time.Sleep(35 * time.Second)
					fullText = ""
					mylogger.AgentStream("Thinking... ")
					continue
				}
				mylogger.Error("Stream error: %v", streamErr)
				break
			}
			break
		}

		fmt.Println() // Newline after stream

		if finalResponse != nil && finalResponse.UsageMetadata != nil {
			um := finalResponse.UsageMetadata
			line := a.tokens.Record(um.PromptTokenCount, um.CandidatesTokenCount)
			mylogger.Tokens(line)
		}

		if fullText == "" && finalResponse == nil {
			return nil
		}

		// Build a clean model Content from the accumulated text and any native
		// function calls. Streamed text providers need fullText, while Gemini
		// needs the function-call parts echoed before function responses.
		modelParts := make([]*genai.Part, 0, 1+len(nativeToolCalls))
		if fullText != "" {
			modelParts = append(modelParts, &genai.Part{Text: fullText})
		}
		for _, call := range nativeToolCalls {
			modelParts = append(modelParts, &genai.Part{FunctionCall: call})
		}
		fullModelContent := &genai.Content{Role: "model", Parts: modelParts}
		a.mem.AddModelContent(fullModelContent)

		// Check for native function calls (Gemini)
		hasToolCall := false
		for _, call := range nativeToolCalls {
			hasToolCall = true
			if a.handleToolCall(call) {
				return nil
			}
		}

		if len(nativeToolCalls) == 0 && finalResponse != nil && len(finalResponse.Candidates) > 0 {
			for _, part := range finalResponse.Candidates[0].Content.Parts {
				if part.FunctionCall != nil {
					hasToolCall = true
					fullModelContent.Parts = append(fullModelContent.Parts, &genai.Part{FunctionCall: part.FunctionCall})
					if a.handleToolCall(part.FunctionCall) {
						return nil
					}
				}
			}
		}

		// Fallback for LLMs that output raw JSON text (extract ALL json blocks)
		if !hasToolCall {
			toolCalls := extractAllJSONCalls(fullText)
			for _, fc := range toolCalls {
				hasToolCall = true
				fullModelContent.Parts = append(fullModelContent.Parts, &genai.Part{FunctionCall: fc})
				if a.handleToolCall(fc) {
					return nil
				}
			}
		}

		if !hasToolCall {
			break // No tools requested, agent is done with this turn
		}
	}

	return nil
}

func (a *Agent) handleToolCall(call *genai.FunctionCall) bool {
	if call.Name == "finish" {
		msg := "Task completed."
		if call.Args != nil {
			if m, ok := call.Args["message"].(string); ok {
				msg = m
			} else if m, ok := call.Args["summary"].(string); ok {
				msg = m
			}
		}
		mylogger.Agent("Agent Finished: %s", msg)
		return true // done
	}

	mylogger.Tool("Calling tool: %s", call.Name)

	result, err := a.exec.Execute(call.Name, call.Args)
	if err != nil {
		mylogger.Error("Tool %s failed: %v", call.Name, err)
		a.mem.AddFunctionResponse(call.Name, map[string]any{"error": err.Error()})
	} else {
		a.mem.AddFunctionResponse(call.Name, result)
	}
	return false
}

// extractAllJSONCalls scans text for all ```json ... ``` blocks and parses
// each one as a tool call. This lets us handle models (like Qwen) that plan
// and output multiple tool calls in a single response.
func extractAllJSONCalls(text string) []*genai.FunctionCall {
	var calls []*genai.FunctionCall
	remaining := text

	for {
		startMarker := "```json"
		startIdx := strings.Index(remaining, startMarker)
		if startIdx == -1 {
			break
		}
		afterOpen := remaining[startIdx+len(startMarker):]
		endIdx := strings.Index(afterOpen, "```")
		if endIdx == -1 {
			break
		}
		block := strings.TrimSpace(afterOpen[:endIdx])

		var call struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		}
		if err := json.Unmarshal([]byte(block), &call); err == nil && call.Name != "" {
			calls = append(calls, &genai.FunctionCall{
				Name: call.Name,
				Args: call.Args,
			})
		}

		// Advance past this block
		remaining = afterOpen[endIdx+3:]
	}

	return calls
}

// TokenSummary returns the session token summary string.
func (a *Agent) TokenSummary() string {
	return a.tokens.Summary()
}

func (a *Agent) PrintSessionSummary() {
	mylogger.TokenSummary(a.tokens.Summary())
}

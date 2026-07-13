package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"os"

	"google.golang.org/genai"
)

type OllamaClient struct {
	model string
	url   string
}

// Ensure OllamaClient implements Provider
var _ Provider = (*OllamaClient)(nil)

func NewOllamaClient() *OllamaClient {
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "gemma2:2b"
	}
	url := os.Getenv("OLLAMA_URL")
	if url == "" {
		url = "http://localhost:11434"
	}
	return &OllamaClient{
		model: modelName,
		url:   url,
	}
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int32         `json:"prompt_eval_count"` // input tokens
	EvalCount       int32         `json:"eval_count"`        // output tokens
}

func (o *OllamaClient) GenerateContentStream(ctx context.Context, history []*genai.Content, systemInstruction string, tools []*genai.Tool) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		var messages []ollamaMessage
		
		if systemInstruction != "" {
			messages = append(messages, ollamaMessage{Role: "system", Content: systemInstruction})
		}

		for _, h := range history {
			role := "user"
			if h.Role == "model" || h.Role == "assistant" {
				role = "assistant"
			}
			
			// Serialize tool calls and their results so the model can see command output.
			var content string
			for _, p := range h.Parts {
				if p.Text != "" {
					content += p.Text
				} else if p.FunctionCall != nil {
					content += fmt.Sprintf("\n[Tool Call: %s]", p.FunctionCall.Name)
				} else if p.FunctionResponse != nil {
					// Marshal the actual response so the model sees stdout, file contents, etc.
					resultBytes, err := json.Marshal(p.FunctionResponse.Response)
					if err != nil {
						content += fmt.Sprintf("\n[Tool Result: %s: (marshal error)]", p.FunctionResponse.Name)
					} else {
						content += fmt.Sprintf("\n[Tool Result: %s]\n%s", p.FunctionResponse.Name, string(resultBytes))
					}
				}
			}
			
			messages = append(messages, ollamaMessage{Role: role, Content: content})
		}

		// Tell the model about tools manually if using standard chat without native tools
		if len(tools) > 0 {
			toolInfo := "\n\nYou have access to the following tools. To use a tool, respond with ONLY a JSON block representing the tool call, e.g. {\"name\": \"read_file\", \"args\": {\"path\": \"...\"}}.\n"
			for _, t := range tools[0].FunctionDeclarations {
				toolInfo += fmt.Sprintf("- %s: %s\n", t.Name, t.Description)
			}
			if len(messages) > 0 && messages[0].Role == "system" {
				messages[0].Content += toolInfo
			} else {
				messages = append([]ollamaMessage{{Role: "system", Content: toolInfo}}, messages...)
			}
		}

		reqBody := ollamaRequest{
			Model:    o.model,
			Messages: messages,
			Stream:   true,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			yield(nil, err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", o.url+"/api/chat", bytes.NewBuffer(jsonData))
		if err != nil {
			yield(nil, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("ollama API error: %d", resp.StatusCode))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var ollamaResp ollamaResponse
			if err := json.Unmarshal(scanner.Bytes(), &ollamaResp); err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}

			// Map back to genai response
			genaiResp := &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Role: "model",
							Parts: []*genai.Part{
								{Text: ollamaResp.Message.Content},
							},
						},
					},
				},
			}

			// The final chunk carries token counts; map them into UsageMetadata
			// so the agent loop can treat Ollama and Gemini identically.
			if ollamaResp.Done {
				genaiResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
					PromptTokenCount:     ollamaResp.PromptEvalCount,
					CandidatesTokenCount: ollamaResp.EvalCount,
					TotalTokenCount:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
				}
			}

			if !yield(genaiResp, nil) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(nil, err)
		}
	}
}

package llm

import (
	"context"
	"iter"
	"os"

	"google.golang.org/genai"
)

type GeminiClient struct {
	client *genai.Client
	model  string
}

// Ensure GeminiClient implements Provider
var _ Provider = (*GeminiClient)(nil)

func NewGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, err
	}

	modelName := os.Getenv("GEMINI_MODEL")
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	return &GeminiClient{
		client: client,
		model:  modelName,
	}, nil
}

func (c *GeminiClient) GenerateContentStream(ctx context.Context, history []*genai.Content, systemInstruction string, tools []*genai.Tool) iter.Seq2[*genai.GenerateContentResponse, error] {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
		Tools: tools,
	}
	
	return c.client.Models.GenerateContentStream(ctx, c.model, history, config)
}

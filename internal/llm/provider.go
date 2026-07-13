package llm

import (
	"context"
	"iter"

	"google.golang.org/genai"
)

type Provider interface {
	GenerateContentStream(ctx context.Context, history []*genai.Content, systemInstruction string, tools []*genai.Tool) iter.Seq2[*genai.GenerateContentResponse, error]
}

package main

import (
	"context"
	"fmt"
	"os"

	"terminal-coding-agent/internal/agent"
	"terminal-coding-agent/internal/cli"
	mylogger "terminal-coding-agent/internal/logger"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Initialize logger
	mylogger.Init()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" && os.Getenv("LLM_PROVIDER") != "ollama" {
		mylogger.Error("GEMINI_API_KEY is not set. Please set it in your environment or a .env file.")
		os.Exit(1)
	}

	mylogger.System("Starting Terminal Coding Agent V1...")

	ctx := context.Background()
	ag, err := agent.NewAgent(ctx, apiKey)
	if err != nil {
		mylogger.Error(fmt.Sprintf("Failed to initialize agent: %v", err))
		os.Exit(1)
	}

	mylogger.System("Agent initialized. Type 'exit' to quit.")

	// Start CLI loop
	cli.Run(ctx, ag)
}

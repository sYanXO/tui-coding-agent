package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"terminal-coding-agent/internal/agent"
	mylogger "terminal-coding-agent/internal/logger"
)

func Run(ctx context.Context, ag *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		mylogger.Printf("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "exit", "quit":
			mylogger.System("Exiting...")
			return
		case "clear":
			fmt.Print("\033[H\033[2J") // Clear terminal
			continue
		case "help":
			mylogger.System("Commands: exit, quit, clear, help")
			continue
		}

		err := ag.HandleUserRequest(ctx, input)
		if err != nil {
			mylogger.Error("Agent encountered an error: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		mylogger.Error("Error reading input: %v", err)
	}
}

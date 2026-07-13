package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mylogger "terminal-coding-agent/internal/logger"
)

type Executor struct {
	workspace string
}

func NewExecutor() (*Executor, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Executor{workspace: cwd}, nil
}

func (e *Executor) Execute(name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "read_file":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid path argument")
		}
		return e.readFile(path)

	case "write_file":
		path, ok1 := args["path"].(string)
		content, ok2 := args["content"].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("missing or invalid path/content arguments")
		}
		return e.writeFile(path, content)

	case "list_directory":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid path argument")
		}
		return e.listDirectory(path)

	case "search_files":
		query, ok := args["query"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid query argument")
		}
		return e.searchFiles(query)

	case "run_bash":
		command, ok := args["command"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid command argument")
		}
		return e.runBash(command)

	case "finish":
		msg, ok := args["message"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid message argument")
		}
		return map[string]any{"status": "completed", "message": msg}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (e *Executor) readFile(path string) (map[string]any, error) {
	mylogger.Tool("Reading file: %s", path)
	content, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"content": string(content)}, nil
}

func (e *Executor) writeFile(path string, content string) (map[string]any, error) {
	mylogger.Tool("Writing file: %s", path)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"status": "success"}, nil
}

func (e *Executor) listDirectory(path string) (map[string]any, error) {
	mylogger.Tool("Listing directory: %s", path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	var files []string
	var folders []string

	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		} else {
			files = append(files, entry.Name())
		}
	}

	return map[string]any{
		"files":   files,
		"folders": folders,
	}, nil
}

func (e *Executor) searchFiles(query string) (map[string]any, error) {
	mylogger.Tool("Searching files for: %s", query)
	var matches []string

	err := filepath.Walk(e.workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors
		}
		if !info.IsDir() && strings.Contains(info.Name(), query) {
			relPath, _ := filepath.Rel(e.workspace, path)
			matches = append(matches, relPath)
		}
		return nil
	})

	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"matches": matches}, nil
}

func (e *Executor) runBash(command string) (map[string]any, error) {
	mylogger.Tool("Running bash: %s", command)

	// Safety checks
	forbidden := []string{"sudo ", "shutdown", "rm -rf /", "cd .."}
	for _, word := range forbidden {
		if strings.Contains(command, word) {
			return map[string]any{"error": "Command contains forbidden string: " + word}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = e.workspace

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return map[string]any{
		"output":    string(output),
		"exit_code": exitCode,
	}, nil
}

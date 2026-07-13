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
	workspace    string
	useSandbox   bool
	sandboxImage string
	sandboxShell string
}

func NewExecutor() (*Executor, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	useSandbox := os.Getenv("DOCKER_SANDBOX") == "true"
	sandboxImage := os.Getenv("DOCKER_IMAGE")
	if sandboxImage == "" {
		sandboxImage = "golang:1.26.5-alpine"
	}

	sandboxShell := os.Getenv("DOCKER_SHELL")
	if sandboxShell == "" {
		if strings.Contains(sandboxImage, "alpine") {
			sandboxShell = "sh"
		} else {
			sandboxShell = "bash"
		}
	}

	return &Executor{
		workspace:    cwd,
		useSandbox:   useSandbox,
		sandboxImage: sandboxImage,
		sandboxShell: sandboxShell,
	}, nil
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
	// Safety checks
	forbidden := []string{"sudo ", "shutdown", "rm -rf /", "cd .."}
	for _, word := range forbidden {
		if strings.Contains(command, word) {
			return map[string]any{"error": "Command contains forbidden string: " + word}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if e.useSandbox {
		mylogger.Tool("Running inside Docker sandbox (%s): %s", e.sandboxImage, command)

		uid := os.Getuid()
		gid := os.Getgid()

		dockerArgs := []string{
			"run", "--rm",
			"-i",
			"-u", fmt.Sprintf("%d:%d", uid, gid),
			"-v", fmt.Sprintf("%s:/workspace", e.workspace),
			"-w", "/workspace",
			"-e", "HOME=/tmp",
		}

		// Mount Go build cache if exists on host
		hostGoCache := os.Getenv("GOCACHE")
		if hostGoCache == "" {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				hostGoCache = filepath.Join(home, ".cache", "go-build")
			}
		}
		if hostGoCache != "" {
			if _, err := os.Stat(hostGoCache); err == nil {
				dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/go-cache", hostGoCache), "-e", "GOCACHE=/go-cache")
			}
		}

		// Mount Go module cache if exists on host
		hostGoModCache := os.Getenv("GOMODCACHE")
		if hostGoModCache == "" {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				hostGoModCache = filepath.Join(home, "go", "pkg", "mod")
			}
		}
		if hostGoModCache != "" {
			if _, err := os.Stat(hostGoModCache); err == nil {
				dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/go-mod-cache", hostGoModCache), "-e", "GOMODCACHE=/go-mod-cache")
			}
		}

		dockerArgs = append(dockerArgs, e.sandboxImage, e.sandboxShell, "-c", command)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
	} else {
		mylogger.Tool("Running bash: %s", command)
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir = e.workspace
	}

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

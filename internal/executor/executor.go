package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"terminal-coding-agent/internal/index"
	mylogger "terminal-coding-agent/internal/logger"
)

type Executor struct {
	workspace    string
	useSandbox   bool
	sandboxImage string
	sandboxShell string
	indexer      *index.Indexer
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

	idx := index.NewIndexer(cwd)
	// Run initial scan to orient the agent on startup
	_ = idx.Scan()

	return &Executor{
		workspace:    cwd,
		useSandbox:   useSandbox,
		sandboxImage: sandboxImage,
		sandboxShell: sandboxShell,
		indexer:      idx,
	}, nil
}

func (e *Executor) Execute(name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "search_symbols":
		query, ok := args["query"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid query argument")
		}
		// Refresh scan before searching to ensure freshness
		_ = e.indexer.Scan()
		results := e.indexer.Search(query)
		return map[string]any{"symbols": results}, nil

	case "list_symbols":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid path argument")
		}
		if !e.isPathInWorkspace(path) {
			return map[string]any{"error": "access denied: path is outside workspace"}, nil
		}
		_ = e.indexer.Scan()
		symbols := e.indexer.ListFile(path)
		return map[string]any{"symbols": symbols}, nil

	case "get_repo_map":
		_ = e.indexer.Scan()
		return e.indexer.GetRepoMap(), nil

	case "read_file":
		path, ok := args["path"].(string)
		if !ok {
			if p, ok2 := args["file_path"].(string); ok2 {
				path = p
				ok = true
			} else if p, ok2 := args["filePath"].(string); ok2 {
				path = p
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("missing or invalid path argument")
		}
		return e.readFile(path)

	case "write_file":
		path, ok1 := args["path"].(string)
		if !ok1 {
			if p, ok := args["file_path"].(string); ok {
				path = p
				ok1 = true
			} else if p, ok := args["filePath"].(string); ok {
				path = p
				ok1 = true
			}
		}
		content, ok2 := args["content"].(string)
		if !ok2 {
			if c, ok := args["file_content"].(string); ok {
				content = c
				ok2 = true
			} else if c, ok := args["fileContent"].(string); ok {
				content = c
				ok2 = true
			}
		}
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("missing or invalid path/content arguments")
		}
		return e.writeFile(path, content)

	case "patch_file":
		path, ok1 := args["path"].(string)
		if !ok1 {
			if p, ok := args["file_path"].(string); ok {
				path = p
				ok1 = true
			} else if p, ok := args["filePath"].(string); ok {
				path = p
				ok1 = true
			}
		}
		search, ok2 := args["search"].(string)
		if !ok2 {
			if s, ok := args["search_pattern"].(string); ok {
				search = s
				ok2 = true
			} else if s, ok := args["searchPattern"].(string); ok {
				search = s
				ok2 = true
			} else if s, ok := args["pattern"].(string); ok {
				search = s
				ok2 = true
			}
		}
		replace, ok3 := args["replace"].(string)
		if !ok3 {
			if r, ok := args["replacement"].(string); ok {
				replace = r
				ok3 = true
			} else if r, ok := args["replace_value"].(string); ok {
				replace = r
				ok3 = true
			}
		}
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("missing or invalid path/search/replace arguments")
		}
		return e.patchFile(path, search, replace)

	case "list_directory":
		path, ok := args["path"].(string)
		if !ok {
			if p, ok2 := args["dir_path"].(string); ok2 {
				path = p
				ok = true
			} else if p, ok2 := args["dirPath"].(string); ok2 {
				path = p
				ok = true
			} else if p, ok2 := args["directory"].(string); ok2 {
				path = p
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("missing or invalid path argument")
		}
		return e.listDirectory(path)

	case "search_files":
		query, ok := args["query"].(string)
		if !ok {
			if q, ok2 := args["search"].(string); ok2 {
				query = q
				ok = true
			} else if q, ok2 := args["pattern"].(string); ok2 {
				query = q
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("missing or invalid query argument")
		}
		return e.searchFiles(query)

	case "run_bash":
		command, ok := args["command"].(string)
		if !ok {
			if c, ok2 := args["cmd"].(string); ok2 {
				command = c
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("missing or invalid command argument")
		}
		return e.runBash(command)

	case "finish":
		msg, ok := args["message"].(string)
		if !ok {
			if m, ok2 := args["summary"].(string); ok2 {
				msg = m
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("missing or invalid message argument")
		}
		return map[string]any{"status": "completed", "message": msg}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (e *Executor) isPathInWorkspace(path string) bool {
	_, ok := e.resolveWorkspacePath(path)
	return ok
}

func (e *Executor) resolveWorkspacePath(path string) (string, bool) {
	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		absPath = filepath.Clean(filepath.Join(e.workspace, path))
	}

	workspaceToUse := e.workspace
	if evalWorkspace, err := filepath.EvalSymlinks(e.workspace); err == nil {
		workspaceToUse = evalWorkspace
	}

	if evalPath, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = evalPath
	} else {
		parent := filepath.Dir(absPath)
		if evalParent, err := filepath.EvalSymlinks(parent); err == nil {
			absPath = filepath.Join(evalParent, filepath.Base(absPath))
		}
	}

	rel, err := filepath.Rel(workspaceToUse, absPath)
	if err != nil {
		return "", false
	}

	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return absPath, true
	}
	return "", false
}

func (e *Executor) readFile(path string) (map[string]any, error) {
	resolvedPath, ok := e.resolveWorkspacePath(path)
	if !ok {
		return map[string]any{"error": "access denied: path is outside workspace"}, nil
	}
	mylogger.Tool("Reading file: %s", path)
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"content": string(content)}, nil
}

func (e *Executor) writeFile(path string, content string) (map[string]any, error) {
	resolvedPath, ok := e.resolveWorkspacePath(path)
	if !ok {
		return map[string]any{"error": "access denied: path is outside workspace"}, nil
	}
	mylogger.Tool("Writing file: %s", path)
	err := os.WriteFile(resolvedPath, []byte(content), 0644)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"status": "success"}, nil
}

func (e *Executor) patchFile(path string, search string, replace string) (map[string]any, error) {
	resolvedPath, ok := e.resolveWorkspacePath(path)
	if !ok {
		return map[string]any{"error": "access denied: path is outside workspace"}, nil
	}
	mylogger.Tool("Patching file: %s", path)

	contentBytes, err := os.ReadFile(resolvedPath)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	content := string(contentBytes)

	// Verify that the search string exists and is unique
	count := strings.Count(content, search)
	if count == 0 {
		return map[string]any{"error": "search block not found in file"}, nil
	}
	if count > 1 {
		return map[string]any{"error": "search block matches multiple times, patch is ambiguous"}, nil
	}

	newContent := strings.Replace(content, search, replace, 1)

	// Print the diff in git-diff style
	e.printDiff(resolvedPath, content, search, replace)

	// Write new content
	err = os.WriteFile(resolvedPath, []byte(newContent), 0644)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	return map[string]any{"status": "success"}, nil
}

func (e *Executor) printDiff(path string, content string, search string, replace string) {
	idx := strings.Index(content, search)
	if idx == -1 {
		return
	}

	before := content[:idx]
	after := content[idx+len(search):]

	var beforeLines []string
	if before != "" {
		beforeLines = strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	}
	var afterLines []string
	if after != "" {
		afterLines = strings.Split(strings.TrimSuffix(strings.TrimPrefix(after, "\n"), "\n"), "\n")
	}

	searchLines := strings.Split(strings.TrimSuffix(search, "\n"), "\n")
	replaceLines := strings.Split(strings.TrimSuffix(replace, "\n"), "\n")

	displayPath := path
	if rel, err := filepath.Rel(e.workspace, path); err == nil {
		displayPath = rel
	}

	mylogger.Printf("--- a/%s\n", displayPath)
	mylogger.Printf("+++ b/%s\n", displayPath)

	startLine := len(beforeLines) + 1

	mylogger.DiffHeader("@@ -%d,%d +%d,%d @@", startLine, len(searchLines), startLine, len(replaceLines))

	// Context before (max 3 lines)
	contextBefore := 3
	if len(beforeLines) < contextBefore {
		contextBefore = len(beforeLines)
	}
	for i := len(beforeLines) - contextBefore; i < len(beforeLines); i++ {
		if i >= 0 && i < len(beforeLines) {
			mylogger.Printf("  %s\n", beforeLines[i])
		}
	}

	// Red removals
	for _, line := range searchLines {
		mylogger.DiffMinus("- %s", line)
	}

	// Green additions
	for _, line := range replaceLines {
		mylogger.DiffPlus("+ %s", line)
	}

	// Context after (max 3 lines)
	contextAfter := 3
	if len(afterLines) < contextAfter {
		contextAfter = len(afterLines)
	}
	for i := 0; i < contextAfter; i++ {
		if i < len(afterLines) {
			mylogger.Printf("  %s\n", afterLines[i])
		}
	}
}

func (e *Executor) listDirectory(path string) (map[string]any, error) {
	resolvedPath, ok := e.resolveWorkspacePath(path)
	if !ok {
		return map[string]any{"error": "access denied: path is outside workspace"}, nil
	}
	mylogger.Tool("Listing directory: %s", path)
	entries, err := os.ReadDir(resolvedPath)
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
	forbidden := []string{"sudo ", "shutdown", "rm -rf /", "cd ..", "reboot"}
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

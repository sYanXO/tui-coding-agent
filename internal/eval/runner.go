package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"terminal-coding-agent/internal/agent"
	"terminal-coding-agent/internal/executor"
)

func Run(ctx context.Context, opts Options) ([]Result, error) {
	root := opts.Root
	if root == "" {
		root = "evals"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	tasks, err := loadTasks(root, opts.Task)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(tasks))
	for _, task := range tasks {
		result := runTask(ctx, root, task, opts)
		results = append(results, result)
		if !opts.KeepWorkspace && result.Workspace != "" {
			_ = os.RemoveAll(result.Workspace)
			result.Workspace = ""
			results[len(results)-1] = result
		}
	}

	return results, nil
}

func loadTasks(root string, selected string) ([]Task, error) {
	if selected != "" {
		path := selected
		if !strings.HasSuffix(path, ".json") {
			path = filepath.Join(root, "tasks", selected+".json")
		} else if !filepath.IsAbs(path) {
			if _, err := os.Stat(path); err != nil {
				path = filepath.Join(root, path)
			}
		}
		task, err := loadTask(path)
		if err != nil {
			return nil, err
		}
		return []Task{task}, nil
	}

	matches, err := filepath.Glob(filepath.Join(root, "tasks", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no eval tasks found under %s", filepath.Join(root, "tasks"))
	}

	tasks := make([]Task, 0, len(matches))
	for _, path := range matches {
		task, err := loadTask(path)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func loadTask(path string) (Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Task{}, err
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return Task{}, err
	}
	if task.ID == "" || task.Prompt == "" || task.Fixture == "" || task.Checks.Command == "" {
		return Task{}, fmt.Errorf("%s is missing id, prompt, fixture, or checks.command", path)
	}
	if task.Script == "" {
		return Task{}, fmt.Errorf("%s is missing script", path)
	}
	return task, nil
}

func runTask(ctx context.Context, root string, task Task, opts Options) Result {
	result := Result{
		TaskID:       task.ID,
		Provider:     "scripted",
		CheckCommand: task.Checks.Command,
	}

	workspace, err := os.MkdirTemp("", "agent-eval-"+task.ID+"-")
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Workspace = workspace

	fixture := filepath.Join(root, task.Fixture)
	if err := copyDir(fixture, workspace); err != nil {
		result.Error = err.Error()
		return result
	}

	provider, err := NewScriptedProvider(filepath.Join(root, task.Script))
	if err != nil {
		result.Error = err.Error()
		return result
	}

	exec, err := executor.NewExecutorForWorkspace(workspace)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	before, err := snapshotWorkspace(workspace)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	ag, err := agent.NewAgentWithOptions(agent.Options{
		Provider:      provider,
		ProviderName:  "scripted",
		Executor:      exec,
		MaxIterations: opts.MaxIterations,
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if err := ag.HandleUserRequest(ctx, task.Prompt); err != nil {
		result.Error = err.Error()
	}

	stats := ag.RunStats()
	result.FinishCalled = stats.FinishCalled
	result.Iterations = stats.Iterations
	result.ToolCalls = stats.ToolCalls
	result.ToolErrors = stats.ToolErrors
	result.ToolCallNames = stats.ToolCallNames

	after, err := snapshotWorkspace(workspace)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.ChangedFiles = diffSnapshots(before, after)

	exitCode, output := runCheck(ctx, workspace, task.Checks.Command)
	result.CheckExitCode = exitCode
	result.CheckOutput = output

	scoreResult(&result, task)
	return result
}

func runCheck(ctx context.Context, workspace string, command string) (int, string) {
	checkCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, "bash", "-c", command)
	cmd.Dir = workspace
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(output)
	}
	return -1, string(output) + err.Error()
}

func scoreResult(result *Result, task Task) {
	var failures []string
	if result.Error != "" {
		failures = append(failures, result.Error)
	}
	if !result.FinishCalled {
		failures = append(failures, "finish was not called")
	}
	if result.CheckExitCode != 0 {
		failures = append(failures, fmt.Sprintf("check command exited %d", result.CheckExitCode))
	}

	changed := make(map[string]bool, len(result.ChangedFiles))
	for _, file := range result.ChangedFiles {
		changed[file] = true
	}
	for _, file := range task.Checks.ExpectedFilesChanged {
		if !changed[file] {
			failures = append(failures, "expected file not changed: "+file)
		}
	}
	for _, file := range task.Checks.ForbiddenFilesChanged {
		if changed[file] {
			failures = append(failures, "forbidden file changed: "+file)
		}
	}

	result.FailureReasons = failures
	result.Passed = len(failures) == 0
	if result.Passed {
		result.Score = 1
	}
}

func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()

		dest, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dest.Close()

		_, err = io.Copy(dest, source)
		return err
	})
}

func snapshotWorkspace(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".agent_index.json" {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = hash
		return nil
	})
	return files, err
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func diffSnapshots(before map[string]string, after map[string]string) []string {
	changed := make(map[string]bool)
	for path, beforeHash := range before {
		if afterHash, ok := after[path]; !ok || afterHash != beforeHash {
			changed[path] = true
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changed[path] = true
		}
	}

	files := make([]string, 0, len(changed))
	for path := range changed {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}

package executor

import (
	"os"
	"strings"
	"testing"

	"terminal-coding-agent/internal/logger"
)

func init() {
	logger.Init()
}

func TestNewExecutor(t *testing.T) {
	os.Setenv("DOCKER_SANDBOX", "true")
	os.Setenv("DOCKER_IMAGE", "golang:1.26.5-alpine")
	os.Setenv("DOCKER_SHELL", "sh")
	defer func() {
		os.Unsetenv("DOCKER_SANDBOX")
		os.Unsetenv("DOCKER_IMAGE")
		os.Unsetenv("DOCKER_SHELL")
	}()

	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}

	if !exec.useSandbox {
		t.Errorf("expected useSandbox to be true")
	}
	if exec.sandboxImage != "golang:1.26.5-alpine" {
		t.Errorf("expected sandboxImage to be golang:1.26.5-alpine, got %s", exec.sandboxImage)
	}
	if exec.sandboxShell != "sh" {
		t.Errorf("expected sandboxShell to be sh, got %s", exec.sandboxShell)
	}
}

func TestExecuteRunBashNonSandbox(t *testing.T) {
	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	exec.useSandbox = false

	res, err := exec.Execute("run_bash", map[string]any{"command": "echo 'hello'"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	output := res["output"].(string)
	if !strings.Contains(output, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", output)
	}
	if res["exit_code"].(int) != 0 {
		t.Errorf("expected exit_code 0, got %v", res["exit_code"])
	}
}

func TestExecuteRunBashSandbox(t *testing.T) {
	// Only run this test if docker is available and we want to test sandbox
	if os.Getenv("RUN_DOCKER_TESTS") != "true" {
		t.Skip("Skipping Docker sandbox test; set RUN_DOCKER_TESTS=true to run")
	}

	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	exec.useSandbox = true
	exec.sandboxImage = "golang:1.26.5-alpine"
	exec.sandboxShell = "sh"

	res, err := exec.Execute("run_bash", map[string]any{"command": "echo 'hello from sandbox'"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if errStr, ok := res["error"].(string); ok {
		t.Fatalf("Sandbox execution returned error: %s", errStr)
	}

	output := res["output"].(string)
	if !strings.Contains(output, "hello from sandbox") {
		t.Errorf("expected output to contain 'hello from sandbox', got %q", output)
	}
}

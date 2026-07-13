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

func TestExecuteRunBashForbiddenReboot(t *testing.T) {
	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	exec.useSandbox = false

	res, err := exec.Execute("run_bash", map[string]any{"command": "reboot now"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	errStr, ok := res["error"].(string)
	if !ok {
		t.Fatalf("expected error block for forbidden reboot command")
	}
	if !strings.Contains(errStr, "forbidden string") {
		t.Errorf("expected forbidden command message, got: %s", errStr)
	}
}

func TestPathInWorkspaceRestriction(t *testing.T) {
	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}

	// Inside workspace should be allowed (even if file does not exist, it fails on os level, not sandbox restriction)
	res, err := exec.Execute("read_file", map[string]any{"path": "nonexistent_file_inside.txt"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr, ok := res["error"].(string)
	if !ok || strings.Contains(errStr, "access denied") {
		t.Errorf("expected standard os error, got: %s", errStr)
	}

	// Outside workspace should be blocked
	res2, err := exec.Execute("read_file", map[string]any{"path": "../outside_file.txt"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr2, ok2 := res2["error"].(string)
	if !ok2 || !strings.Contains(errStr2, "access denied") {
		t.Errorf("expected access denied error, got: %v", res2)
	}

	// Write tool outside workspace should be blocked
	res3, err := exec.Execute("write_file", map[string]any{"path": "/etc/passwd", "content": "hack"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr3, ok3 := res3["error"].(string)
	if !ok3 || !strings.Contains(errStr3, "access denied") {
		t.Errorf("expected access denied error on write, got: %v", res3)
	}
}

func TestExecutePatchFile(t *testing.T) {
	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}

	tempFile := "temp_patch_test.txt"
	defer os.Remove(tempFile)

	initialContent := "hello world\nthis is unique line\nhello world\n"
	err = os.WriteFile(tempFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 1. Success patch
	res, err := exec.Execute("patch_file", map[string]any{
		"path":    tempFile,
		"search":  "this is unique line",
		"replace": "this is modified unique line",
	})
	if err != nil {
		t.Fatalf("execute patch failed: %v", err)
	}
	if status, ok := res["status"].(string); !ok || status != "success" {
		t.Fatalf("expected status success, got: %v", res)
	}

	// Read and verify
	data, _ := os.ReadFile(tempFile)
	if !strings.Contains(string(data), "this is modified unique line") {
		t.Errorf("patch was not applied, file content: %s", string(data))
	}

	// 2. Fails: not found
	res2, err := exec.Execute("patch_file", map[string]any{
		"path":    tempFile,
		"search":  "nonexistent search pattern",
		"replace": "foo",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr2, ok2 := res2["error"].(string)
	if !ok2 || !strings.Contains(errStr2, "not found") {
		t.Errorf("expected search not found error, got: %v", res2)
	}

	// 3. Fails: ambiguous (matches "hello world" twice)
	res3, err := exec.Execute("patch_file", map[string]any{
		"path":    tempFile,
		"search":  "hello world",
		"replace": "hello space",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr3, ok3 := res3["error"].(string)
	if !ok3 || !strings.Contains(errStr3, "ambiguous") {
		t.Errorf("expected ambiguous patch error, got: %v", res3)
	}

	// 4. Fails: outside workspace boundary
	res4, err := exec.Execute("patch_file", map[string]any{
		"path":    "../outside_patch.txt",
		"search":  "foo",
		"replace": "bar",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	errStr4, ok4 := res4["error"].(string)
	if !ok4 || !strings.Contains(errStr4, "access denied") {
		t.Errorf("expected access denied error, got: %v", res4)
	}
}

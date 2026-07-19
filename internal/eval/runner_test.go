package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"terminal-coding-agent/internal/logger"
)

func init() {
	logger.Init()
}

func TestRunScriptedEvalEndToEnd(t *testing.T) {
	root := t.TempDir()
	writeTestEvalRoot(t, root)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "go-build"))

	results, err := Run(context.Background(), Options{
		Root: root,
		Task: "fix-go-test-basic",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Passed {
		t.Fatalf("expected eval to pass, failures: %v, output: %s", result.FailureReasons, result.CheckOutput)
	}
	if result.Score != 1 {
		t.Fatalf("expected score 1, got %v", result.Score)
	}
	if !result.FinishCalled {
		t.Fatalf("expected finish to be called")
	}
	if result.CheckExitCode != 0 {
		t.Fatalf("expected check exit code 0, got %d", result.CheckExitCode)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "math.go" {
		t.Fatalf("expected only math.go to change, got %v", result.ChangedFiles)
	}

	fixtureMath, err := os.ReadFile(filepath.Join(root, "fixtures", "go-basic", "math.go"))
	if err != nil {
		t.Fatalf("failed to read source fixture: %v", err)
	}
	if string(fixtureMath) != "package evalfixture\n\nfunc Add(a, b int) int {\n\treturn a - b\n}\n" {
		t.Fatalf("source fixture was modified: %s", string(fixtureMath))
	}
}

func TestScoreFailsWithoutFinish(t *testing.T) {
	result := Result{
		CheckExitCode: 0,
		ChangedFiles:  []string{"math.go"},
	}
	task := Task{
		Checks: Checks{ExpectedFilesChanged: []string{"math.go"}},
	}

	scoreResult(&result, task)

	if result.Passed {
		t.Fatalf("expected score to fail without finish")
	}
	if result.Score != 0 {
		t.Fatalf("expected score 0, got %v", result.Score)
	}
}

func TestScoreFailsForbiddenFileChange(t *testing.T) {
	result := Result{
		FinishCalled:  true,
		CheckExitCode: 0,
		ChangedFiles:  []string{"go.mod", "math.go"},
	}
	task := Task{
		Checks: Checks{
			ExpectedFilesChanged:  []string{"math.go"},
			ForbiddenFilesChanged: []string{"go.mod"},
		},
	}

	scoreResult(&result, task)

	if result.Passed {
		t.Fatalf("expected score to fail when forbidden file changes")
	}
}

func writeTestEvalRoot(t *testing.T, root string) {
	t.Helper()

	mustMkdir(t, filepath.Join(root, "tasks"))
	mustMkdir(t, filepath.Join(root, "scripts"))
	mustMkdir(t, filepath.Join(root, "fixtures", "go-basic"))

	mustWrite(t, filepath.Join(root, "tasks", "fix-go-test-basic.json"), `{
  "id": "fix-go-test-basic",
  "description": "Fix a failing Go unit test with a targeted patch.",
  "prompt": "Fix the failing tests.",
  "fixture": "fixtures/go-basic",
  "script": "scripts/fix-go-test-basic.json",
  "checks": {
    "command": "go test ./...",
    "expected_files_changed": ["math.go"],
    "forbidden_files_changed": ["go.mod"]
  }
}`)

	mustWrite(t, filepath.Join(root, "scripts", "fix-go-test-basic.json"), `[
  {"tool_calls": [{"name": "run_bash", "args": {"command": "go test ./..."}}]},
  {"tool_calls": [{"name": "patch_file", "args": {"path": "math.go", "search": "return a - b", "replace": "return a + b"}}]},
  {"tool_calls": [{"name": "run_bash", "args": {"command": "go test ./..."}}]},
  {"tool_calls": [{"name": "finish", "args": {"message": "done"}}]}
]`)

	mustWrite(t, filepath.Join(root, "fixtures", "go-basic", "go.mod"), "module evalfixture\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "fixtures", "go-basic", "math.go"), "package evalfixture\n\nfunc Add(a, b int) int {\n\treturn a - b\n}\n")
	mustWrite(t, filepath.Join(root, "fixtures", "go-basic", "math_test.go"), "package evalfixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif got := Add(2, 3); got != 5 {\n\t\tt.Fatalf(\"Add(2, 3) = %d, want 5\", got)\n\t}\n}\n")
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

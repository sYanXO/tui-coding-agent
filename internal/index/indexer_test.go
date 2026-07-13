package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFile(t *testing.T) {
	// Create a temporary Go file for testing
	tempDir, err := os.MkdirTemp("", "test_indexer")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package testpkg

import "fmt"

const MyConst = "hello"

var MyVar int = 42

type MyStruct struct {
	Field string
}

type MyInterface interface {
	DoSomething() error
}

func MyFunction(a string) int {
	return 1
}

func (m *MyStruct) MyMethod() {
	fmt.Println("method")
}
`

	tempFile := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(tempFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	symbols, err := parseGoFile(tempFile)
	if err != nil {
		t.Fatalf("parseGoFile failed: %v", err)
	}

	expectedSymbols := map[string]string{
		"MyConst":     "constant",
		"MyVar":       "variable",
		"MyStruct":    "struct",
		"MyInterface": "interface",
		"MyFunction":  "function",
		"MyMethod":    "method",
	}

	if len(symbols) != len(expectedSymbols) {
		t.Errorf("expected %d symbols, got %d", len(expectedSymbols), len(symbols))
	}

	for _, sym := range symbols {
		expectedKind, ok := expectedSymbols[sym.Name]
		if !ok {
			t.Errorf("unexpected symbol found: %s", sym.Name)
			continue
		}
		if sym.Kind != expectedKind {
			t.Errorf("symbol %s: expected kind %q, got %q", sym.Name, expectedKind, sym.Kind)
		}

		// Verify signature looks correct
		switch sym.Name {
		case "MyFunction":
			expectedSig := "func MyFunction(a string) int"
			if sym.Signature != expectedSig {
				t.Errorf("MyFunction: expected signature %q, got %q", expectedSig, sym.Signature)
			}
		case "MyMethod":
			expectedSig := "func (m *MyStruct) MyMethod()"
			if sym.Signature != expectedSig {
				t.Errorf("MyMethod: expected signature %q, got %q", expectedSig, sym.Signature)
			}
		}
	}
}

func TestIndexer_ScanAndSearch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_indexer_scan")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package main
func TargetFunc() {}
`
	tempFile := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(tempFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	indexer := NewIndexer(tempDir)
	if err := indexer.Scan(); err != nil {
		t.Fatalf("indexer.Scan failed: %v", err)
	}

	results := indexer.Search("TargetFunc")
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}

	if results[0]["name"] != "TargetFunc" {
		t.Errorf("expected symbol name 'TargetFunc', got %q", results[0]["name"])
	}
	if results[0]["kind"] != "function" {
		t.Errorf("expected symbol kind 'function', got %q", results[0]["kind"])
	}
	if results[0]["file"] != "main.go" {
		t.Errorf("expected file 'main.go', got %q", results[0]["file"])
	}
}

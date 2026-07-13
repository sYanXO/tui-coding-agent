package prompt

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetSystemInstruction(t *testing.T) {
	// Mock osGetwd to return a known path for testing
	oldOsGetwd := osGetwd
	osGetwd = func() (string, error) {
		return "/mock/path", nil
	}
	defer func() { osGetwd = oldOsGetwd }()

	expectedSubstring := fmt.Sprintf("Current Working Directory: %s", "/mock/path")

	actual := GetSystemInstruction()

	if !strings.Contains(actual, expectedSubstring) {
		t.Errorf("GetSystemInstruction() did not return the expected instruction.\nExpected substring: %s\nActual: %s", expectedSubstring, actual)
	}
}

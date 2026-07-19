package eval

type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Fixture     string `json:"fixture"`
	Script      string `json:"script,omitempty"`
	Checks      Checks `json:"checks"`
}

type Checks struct {
	Command               string   `json:"command"`
	ExpectedFilesChanged  []string `json:"expected_files_changed,omitempty"`
	ForbiddenFilesChanged []string `json:"forbidden_files_changed,omitempty"`
}

type Result struct {
	TaskID         string   `json:"task_id"`
	Provider       string   `json:"provider"`
	Passed         bool     `json:"passed"`
	Score          float64  `json:"score"`
	Error          string   `json:"error,omitempty"`
	Workspace      string   `json:"workspace,omitempty"`
	FinishCalled   bool     `json:"finish_called"`
	Iterations     int      `json:"iterations"`
	ToolCalls      int      `json:"tool_calls"`
	ToolErrors     int      `json:"tool_errors"`
	ToolCallNames  []string `json:"tool_call_names,omitempty"`
	CheckCommand   string   `json:"check_command,omitempty"`
	CheckExitCode  int      `json:"check_exit_code"`
	CheckOutput    string   `json:"check_output,omitempty"`
	ChangedFiles   []string `json:"changed_files"`
	FailureReasons []string `json:"failure_reasons,omitempty"`
}

type Options struct {
	Root          string
	Task          string
	KeepWorkspace bool
	MaxIterations int
}

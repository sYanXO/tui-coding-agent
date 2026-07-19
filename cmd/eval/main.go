package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"terminal-coding-agent/internal/eval"
	mylogger "terminal-coding-agent/internal/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	mylogger.Init()

	var opts eval.Options
	flag.StringVar(&opts.Root, "root", "evals", "eval root directory")
	flag.StringVar(&opts.Task, "task", "", "task id or task JSON path")
	flag.BoolVar(&opts.KeepWorkspace, "keep-workspace", false, "keep temp eval workspaces")
	flag.IntVar(&opts.MaxIterations, "max-iterations", 15, "agent max iterations per task")
	flag.Parse()

	results, err := eval.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval failed: %v\n", err)
		os.Exit(1)
	}

	passed := 0
	for _, result := range results {
		status := "FAIL"
		if result.Passed {
			status = "PASS"
			passed++
		}
		fmt.Printf("%s %s score=%.1f provider=%s iterations=%d tools=%d errors=%d\n",
			status,
			result.TaskID,
			result.Score,
			result.Provider,
			result.Iterations,
			result.ToolCalls,
			result.ToolErrors,
		)
		if len(result.FailureReasons) > 0 {
			for _, reason := range result.FailureReasons {
				fmt.Printf("  - %s\n", reason)
			}
		} else if result.Error != "" {
			fmt.Printf("  - %s\n", result.Error)
		}
		if len(result.ChangedFiles) > 0 {
			fmt.Printf("  changed: %v\n", result.ChangedFiles)
		}
		if opts.KeepWorkspace && result.Workspace != "" {
			fmt.Printf("  workspace: %s\n", result.Workspace)
		}
	}

	fmt.Printf("\n%d/%d evals passed\n", passed, len(results))
	if passed != len(results) {
		os.Exit(1)
	}
}

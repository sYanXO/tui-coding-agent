# Coding Agent V1 Specification

## Project Name

Terminal Coding Agent 

---

# Objective

Build a terminal-based autonomous coding agent capable of understanding a software project, executing commands, modifying files, and iteratively solving programming tasks using an LLM provider.

The focus is on agent orchestration + frontend/UI.

---

# Goals

The agent should be able to:

- Understand a user's coding request.
- Inspect the current repository.
- Read project files.
- Execute shell commands.
- Modify source code.
- Iterate until the task is complete.
- Explain what it changed.
- Run deterministic scripted eval tasks against fixture repositories.

Example:

```
> Fix the failing tests.

Thinking...

Running go test...

Found 3 failing tests.

Reading auth.go...

Editing auth.go...

Running go test...

All tests passed.

Done.
```

---

# Technology Stack

Language:
- Go

LLM:
- Gemini API
- Ollama local models

Interface:
- Terminal (CLI)

Version Control:
- Git

Operating System:
- Linux

---

# Non Goals (V1)

Do NOT implement:

- GUI
- Web interface
- Git commits
- Plugin system
- MCP
- Authentication
- Cloud deployment

---

# Architecture

```
User

↓

CLI

↓

Agent

↓

LLM Provider

├── Gemini
├── Ollama

↓

Tool Dispatcher

├── Read File
├── Write File
├── Patch File
├── List Directory
├── Search Files
├── Symbol Index
├── Bash Executor

↓

Operating System
```

---

# Core Components

## CLI

Responsibilities:

- Interactive prompt
- Read user input
- Display streamed responses
- Display tool execution logs

Commands:

```
exit
clear
help
```

---

## Agent

Responsibilities:

- Maintain conversation history
- Compact older history during long sessions
- Send requests to Gemini
- Execute tool calls
- Continue until completion

Stopping conditions:

- finish()
- Maximum iterations reached
- Fatal error

---

## Gemini Client

Responsibilities:

- Send prompts
- Receive responses
- Handle tool/function calls
- Stream output

---

## Tool Dispatcher

Receives tool calls from Gemini.

Routes execution to the correct tool.

---

# Tools

## read_file

Input

```
path
```

Output

File contents.

---

## write_file

Input

```
path
content
```

Creates or overwrites a file.

---

## list_directory

Input

```
path
```

Returns:

- files
- folders

---

## search_files

Input

```
query
```

Returns matching file paths.

Implementation may simply search filenames in V1.

---

## run_bash

Input

```
command
```

Returns

- stdout
- stderr
- exit code

Safety:

- timeout (30 seconds)
- working directory restriction

---

## finish

Input

```
message
```

Signals completion.

---

# Agent Loop

```
Receive user prompt

↓

Send to Gemini

↓

Gemini requests tool?

↓

YES

↓

Execute tool

↓

Return tool output

↓

Gemini

↓

Tool?

↓

YES

↓

Repeat

↓

NO

↓

finish()

↓

Exit loop
```

---

# Logging

The terminal should clearly display actions.

Example

```
Thinking...

Running:

go test ./...

Reading:

internal/auth.go

Writing:

internal/auth.go

Running:

go test ./...

Done.
```

---

# Context

Each request should include:

- Current working directory
- Conversation history
- Compact summary of older history when needed
- Tool outputs
- User prompt

---

# Safety

Must prevent:

- sudo
- shutdown
- reboot
- rm -rf /
- commands outside working directory

Every bash command must have:

- timeout
- captured stdout
- captured stderr
- exit code

---

# Error Handling

If a tool fails:

- return the error to Gemini
- allow Gemini to recover

Do not crash the application.

---

# Directory Structure

```
cmd/
    main.go

internal/

    agent/

    cli/

    executor/

    llm/

    logger/

    memory/

    prompt/

    tools/

go.mod
```

---

# Milestones

## Milestone 1

- CLI
- Gemini connection
- Simple chat

---

## Milestone 2

- Tool calling
- Read files
- List directories

---

## Milestone 3

- Bash execution

---

## Milestone 4

- File writing

---

## Milestone 5

- Full autonomous agent loop

---

## Milestone 6

- Logging
- Safety checks
- Error handling

---

# Success Criteria

The following interaction should work:

```
$ agent

> Explain this repository.

> Find why tests are failing.

> Fix the failing tests.

> Add JWT authentication.

> Refactor this package.

> Optimize this SQL query.
```

without requiring manual intervention.

---

# Future Improvements (Out of Scope)

- Git integration
- Patch-based editing
- Diff generation
- Docker sandbox
- Repository indexing
- Token accounting
- Multiple LLM providers
- Configuration files
- Plugin architecture
- Parallel tool execution
- Local model support
- TUI using Bubble Tea
- Persistent memory

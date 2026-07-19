# Terminal Coding Agent

A local terminal coding agent written in Go. It can inspect a repository, call tools, edit files, run commands, and loop until it finishes a coding task.

## What It Does

- Runs as an interactive CLI.
- Supports Gemini through the official Go SDK.
- Supports Ollama for local models through `/api/chat`.
- Streams model output to the terminal.
- Executes native Gemini function calls.
- Extracts JSON tool calls from local model text responses.
- Executes multiple tool calls from one model response.
- Keeps conversation and tool-result history across a session.
- Tracks per-turn and session token usage when the provider reports it.
- Retries Gemini 429 rate-limit errors.
- Reads files inside the workspace.
- Writes files inside the workspace.
- Applies exact search-and-replace patches with ambiguity checks.
- Prints patch diffs before writing changes.
- Lists directories.
- Searches filenames.
- Runs shell commands with a 30-second timeout.
- Optionally runs shell commands inside Docker.
- Indexes Go symbols for repository navigation.
- Lists symbols in a Go file.
- Searches symbols by name.

## What It Cannot Yet Do

- It is not a general security sandbox unless `DOCKER_SANDBOX=true` is enabled.
- Host shell commands can still access normal user permissions outside the repo.
- It does not create git commits or pull requests.
- It does not have a GUI or web interface.
- It does not support MCP or plugins.
- It does not authenticate users.
- It does not deploy code.
- It does not parse symbols for languages other than Go.
- It does not manage long-running background commands.
- Local model tool use depends on the model following the JSON tool-call format.
- It does not compact long conversation history yet.

## Setup

Use Gemini:

```sh
export GEMINI_API_KEY=...
go run ./cmd
```

Use Ollama:

```sh
export LLM_PROVIDER=ollama
export OLLAMA_MODEL=qwen2.5-coder:7b
go run ./cmd
```

Enable Docker command isolation:

```sh
export DOCKER_SANDBOX=true
go run ./cmd
```

Optional environment variables:

- `GEMINI_MODEL` defaults to `gemini-2.5-flash`.
- `OLLAMA_URL` defaults to `http://localhost:11434`.
- `DOCKER_IMAGE` defaults to `golang:1.26.5-alpine`.
- `DOCKER_SHELL` defaults to `sh` for Alpine images and `bash` otherwise.

## CLI Commands

- `help`
- `clear`
- `exit`
- `quit`

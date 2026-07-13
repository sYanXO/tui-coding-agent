# Terminal Coding Agent — FAFO Report

> *Fuck Around and Find Out: building a local autonomous coding agent from scratch in an afternoon.*

We took a spec, picked Go + Gemini, and iterated until a small language model running on a laptop CPU could autonomously write code, run tests, fix its own mistakes, and say "Done." — without touching the internet.

---

## What It Currently Supports

**Two LLM backends, one switch:**
- `Gemini 2.5 Flash` — cloud, fast, smart, free tier with auto-retry on rate limits
- `Ollama` (any local model) — fully offline, no API key, tested with `qwen2.5-coder:7b` and `llama3.2`

**Agent tools:**
- `read_file` — read any file in the workspace
- `write_file` — create or overwrite files
- `patch_file` — surgical search-and-replace edits with live git-diff style output (red/green)
- `list_directory` — list files and folders at a path
- `search_files` — search filenames across the workspace
- `run_bash` — execute shell commands with a 30-second timeout
- `finish` — signal task completion with a summary

**Agent loop behaviour:**
- Streams responses to the terminal in real time (coloured output)
- Executes all tool calls from a single response in sequence (handles models that batch-plan)
- Parses both native Gemini `FunctionCall` objects and raw ` ```json ``` ` blocks from local models
- Feeds tool results back into conversation history and keeps looping autonomously
- Stops on `finish`, fatal error, or after 15 iterations
- Retries automatically on Gemini 429 rate limit errors (35s backoff, 3 attempts)

**Safety & Sandboxing:**
- Optional Docker Sandbox for secure shell execution (`run_bash`).
- Active safety checks (blocking `sudo`, `shutdown`, `rm -rf /`, `cd ..` on host).
- Mounts workspace and caches (`GOCACHE`, `GOMODCACHE`) with host user permissions (`-u $(id -u):$(id -g)`) to avoid permission issues and optimize compilation speed.

**CLI commands:** `exit`, `quit`, `clear`, `help`



---

## What We Actually Found Out

### 1. The architecture matters more than the model

The agent loop (read → think → call tool → read result → repeat) is just Go code. The model is just a plug-in. We proved this by swapping Gemini Flash for a 2-billion parameter local model with a single environment variable and watching the same loop keep working. The loop doesn't care what's generating the JSON.

### 2. Free tier Gemini hits walls fast in agent loops

Gemini 2.5 Pro ran out of quota on the very first multi-step task. Agents burn tokens quickly because every tool result gets appended back into the conversation and sent again. The fix: switch to Gemini 2.5 Flash (much higher free limits) and add automatic 429 retry logic with a 35-second backoff.

### 3. Streaming + tool calling is a two-phase problem

When you stream a response token by token, the last chunk is not the full message — it's just the final fragment. If you save that to memory as the "model response", the next turn gets garbage context and the model starts hallucinating. You have to accumulate the full text across all stream events and save that as one clean message. This was the single most impactful bug fix in the whole project.

### 4. Local models don't use structured function calling — they just talk

Gemini supports native function calling: it returns a structured `FunctionCall` object that you can dispatch directly. Local models (Llama 3.2, Qwen 2.5 Coder) instead output raw text that *looks like* JSON inside a markdown block. We had to build a fallback parser that scans the response text for ` ```json ``` ` blocks and treats valid ones as tool calls. Once we did that, local models worked.

### 5. Local models plan everything upfront and batch their tool calls

Gemini tends to call one tool per turn. Qwen 2.5 Coder dumps its entire plan in a single response — four ` ```json ``` ` blocks in a row. If you only extract the first block, you silently skip the rest. The fix is scanning the entire response and executing every valid JSON block in sequence, in order, before going back to the model.

### 6. A 7B model on a CPU i7 is genuinely usable

`qwen2.5-coder:7b` at 4-bit quantization fits in ~5GB of RAM on a Dell Latitude 7490 with 16GB. It runs at roughly 4–6 tokens per second — slow but not painful. It correctly wrote a factorial function, wrote its own test file, ran `go test ./...`, and called `finish`. Fans spin. Laptop survives.

### 7. Smaller models make dumber tool decisions, not wrong ones

Llama 3.2 (3B) used `read_file` on a directory path. It's not broken — it just doesn't know that `read_file` doesn't work on directories. The executor returned an error, the agent fed it back to the model, and the model (mostly) recovered. The agent loop is resilient to model mistakes because errors are just more context.

### 8. Injecting tools as plain text is good enough

We described all tools (name, description, expected JSON format) inside the System Instruction. No special API. No native function schemas passed to the local model. Just a paragraph of text that says "to call a tool, output a JSON block like this." Qwen read it and followed it. This means you can add any new tool by writing two lines of text — no schema wrangling required.

### 9. The `finish` tool is the most important tool

Without a termination signal, the agent loops forever. The model needs an explicit "I am done" action, and we need to detect and honour it. The `finish` tool is just a no-op function that returns `true` to the loop, which breaks it. Without this, the agent would keep calling tools until it hit the max-iteration guard.

### 10. Safety is string matching and that's fine for V1

`sudo`, `rm -rf /`, `shutdown` — all caught by a simple `strings.Contains` check before the command hits `exec.Command`. The working directory is locked to the project root via `cmd.Dir`. It's not a sandbox. It's a speed bump. For a personal dev tool it's enough.

### 11. Local Models Hallucinate Tool Results Before the Tool Runs

**Problem:** When asked to patch a file, `qwen2.5-coder:7b` would output a JSON tool call block and then immediately continue generating text that *fabricated* the tool's result — printing `Report: The file has been patched. New contents: ...` before the tool had actually executed a single line of Go code. The hallucinated output would often be subtly wrong (wrong file content, wrong line numbers), and because the model had already "seen" a fake result in its own output, it would treat the task as done and call `finish` — sometimes without the real tool ever running.

**Why it happens:** These models are trained on agentic traces where the tool call JSON and the tool result appear close together. The model learned to complete that pattern by predicting the result, rather than waiting for external execution. It's not a bug in the model — it's a boundary problem: the model doesn't know where its output ends and the system's response begins.

**Bottleneck:** There is no reliable way to force a partially-streaming local model to stop mid-response. Unlike Gemini (which returns a structured `FunctionCall` object that terminates the text stream), local models via Ollama just keep generating tokens. By the time we detect a `{"name": "patch_file", ...}` block in the stream, several more fabricated lines have already been generated and appended to `fullText`.

**Fix 1 — System Prompt (temp fix):** Updated the system instruction to explicitly forbid predicting tool outputs: *"Once you write a tool call block, stop generating text immediately and wait for the system to execute it. Never predict, simulate, or fabricate tool results."* This reduced hallucination frequency significantly on `qwen2.5-coder:7b`, though it is not 100% reliable — a sufficiently pattern-driven model will still occasionally slip through.

**Fix 2 — Error Resilience:** The real defence is the agent loop itself. When a tool is actually executed and returns an error (e.g. `search block not found` because the model already applied the patch in an earlier real call), that error is fed back into the conversation as the true tool result. The model then reads the real state of the file and self-corrects. Hallucinated output pollutes the context but doesn't corrupt the file system — the tool executor is the ground truth.

### 12. `write_file` Is Too Blunt — `patch_file` Fixes Targeted Edits

The original `write_file` tool overwrites the entire file. For any edit task, the model has to read the whole file, mentally apply the change, and re-emit the complete new content. On a large file this burns tokens, introduces transcription errors, and loses the developer's ability to see what actually changed.

`patch_file` takes a `search` block and a `replace` block. The executor validates the search string exists exactly once (returns `search block matches multiple times, patch is ambiguous` if not), applies the replacement, and then prints a unified diff to the terminal in real time — cyan hunk header, red removals, green additions, 3 lines of surrounding context — before writing the file. Developers immediately see what changed, in the same visual language as `git diff`.

### 13. Local Models Ignore Your Schema — Parameter Aliasing Saves the Day

Even with a well-defined tool schema (`path`, `search`, `replace`), `qwen2.5-coder:7b` consistently called `patch_file` with its own preferred argument names: `file_path`, `search_pattern`, and `replacement`. The dispatcher returned `missing or invalid arguments` and the tool never ran.

The fix is alias resolution in the executor's dispatch layer. Before failing, each tool now checks a ranked list of alternative key names and promotes the first match to the canonical parameter. This costs two extra map lookups per tool call and makes the agent work correctly with any model that has a reasonable idea of what the argument means, regardless of what it chose to name it.

---

## What We Built, In Order

1. Scaffolded the Go project and connected Gemini Flash via the official SDK.
2. Built a CLI with a `bufio.Scanner` loop and colored output via `fatih/color`.
3. Defined the six tools as Gemini function schemas and wired up the dispatcher.
4. Built the agent loop: send history → stream response → detect tool calls → execute → append result → repeat.
5. Hit the 429 wall. Added retry logic.
6. Fixed the streaming memory bug (saving last chunk instead of full text).
7. Added the `Provider` interface so Gemini and Ollama share the same loop.
8. Built the Ollama client using its `/api/chat` streaming HTTP API.
9. Added the raw JSON fallback parser for local models.
10. Upgraded to multi-block parsing so Qwen's batched plans fully execute.
11. Tested on `qwen2.5-coder:7b` running locally. It worked.
12. Implemented Docker sandboxing for the `run_bash` tool with UID/GID permission mapping and Go build/module cache mounts.
13. Added robust shell detection (falling back to `sh` when `bash` is unavailable, e.g., on Alpine).
14. Fixed tool result serialization in Ollama client to forward actual outputs back to the model.
15. Added workspace path boundary enforcement to `read_file`, `write_file`, `list_directory` — blocks traversal outside the project root.
16. Added `reboot` to the forbidden command list in `run_bash` (was missing from the original spec implementation).
17. Implemented `patch_file`: surgical search-and-replace with uniqueness validation, ambiguity detection, and live git-diff style terminal output (coloured red/green).
18. Added parameter aliasing to the tool dispatcher so local models using non-canonical argument names (`file_path`, `search_pattern`, `replacement`, etc.) still work correctly.
19. Updated system prompt to explicitly prevent local models from hallucinating fabricated tool results before execution.

---

## Stack

- **Language:** Go
- **Cloud model:** Gemini 2.5 Flash (via `google.golang.org/genai`)
- **Local model:** Qwen 2.5 Coder 7B (via Ollama's HTTP API)
- **Switching between them:** `export LLM_PROVIDER=ollama`

## Docker Sandboxing

To isolate shell commands executed by the agent, you can enable Docker sandboxing:
1. Set `DOCKER_SANDBOX=true` in your environment or `.env` file.
2. (Optional) Customize the image using `DOCKER_IMAGE` (defaults to `golang:1.26.5-alpine`).
3. (Optional) Customize the shell using `DOCKER_SHELL` (defaults to `sh` for Alpine-based images, and `bash` otherwise).

This mounts your workspace and leverages host Go build and module caches for near-zero compilation overhead, running commands securely as the host user to prevent permission issues.

# Statix Agent Instructions

This file applies to the entire repository. Every AI agent and delegated subagent MUST read this file completely before inspecting, editing, testing, committing, or pushing repository changes. A delegating agent MUST include that requirement in every subagent task.

## 1. Think Before Coding

- Do not assume silently. State material assumptions and surface tradeoffs.
- Resolve discoverable facts from the repository before asking the user.
- If requirements have multiple materially different interpretations, stop and clarify.
- Prefer the smallest design that fully meets the requested behavior.
- Define verifiable success criteria before editing.

## 2. Keep Changes Simple and Surgical (Ponytail Principle)

- Touch only files and lines required by the task.
- Do not refactor, reformat, rename, or delete adjacent code without a direct need.
- Reuse existing helpers, Go standard library (`net`, `crypto`, `sync`, `time`, `bufio`), and installed dependencies before adding new abstractions or third-party packages.
- Do not create interfaces, factories, configuration layers, or extension points for a single implementation.
- Remove imports, variables, and helpers made unused by your own change.
- Never delete pre-existing code merely because it appears unused; mention unrelated problems separately.
- Preserve user-authored changes in a dirty worktree. Never run destructive Git recovery commands (`git reset --hard`, `git clean -fd`) unless explicitly authorized.

## 3. Repository Architecture & Orientation

- `cmd/statix/`: Main application entrypoint, CLI flag parsing, graceful OS signal handling, listener binding with automatic port fallback.
- `internal/auth/`: Session management, Argon2id password hashing, sliding-window rate limiting, and double-submit CSRF protection middleware.
- `internal/config/`: Configuration struct, defaults, JSON persistence, and validation.
- `internal/metrics/`: Telemetry collection (`/proc` parsers for Linux CPU, Mem, Disk, Net, Processes), in-memory circular `RingBuffer`, and background collector.
- `internal/platform/`: OS and platform-specific helpers (`/proc` file scanners, `statfs`).
- `internal/webui/`: Web dashboard server (Chi router, embedded templates/assets, HTMX, Alpine.js, uPlot, and real-time WebSocket hub).
- `.github/workflows/`: CI test and lint workflow (`ci.yml`) and immutable GitHub release workflow (`release.yml`).

## 4. Concurrency & Goroutine Discipline

- **Lifecycle and Context:** Every long-running goroutine must accept a `context.Context` and terminate promptly upon `ctx.Done()`. Never leave orphaned or unbounded background goroutines.
- **Shutdown Safety:** Background hubs (e.g. `WSHub`) and collectors must provide safe shutdown semantics:
  - Channels must never block indefinitely on shutdown; use non-blocking `select` with `case <-h.done:` or `default:`.
  - Close underlying network connections immediately (`CloseNow()`) during shutdown to prevent blocking on TCP close handshakes.
- **Data Race Prevention & Defensive Copying:** Slices and pointers shared across goroutines (such as `Snapshot` arrays for CPU, Disks, Networks, Processes) must be defensively cloned (`Clone()`) before returning from concurrent data structures like `RingBuffer.Latest()` or `RingBuffer.All()`.
- **Mutex Scoping:** Keep critical sections minimal. Never perform blocking I/O, network writes, or lock acquisitions while holding shared mutexes.

## 5. Security & Web Standards

- **Cross-Site WebSocket Hijacking (CSWSH):** WebSocket upgrades must strictly validate the `Origin` header against the request's `Host` header. Never set `InsecureSkipVerify: true` or `OriginPatterns: []string{"*"}` in production endpoints.
- **CSRF Protection:** Mutating HTTP requests (POST, PUT, DELETE, PATCH) require strict double-submit cookie verification. Never implement auto-repair or bypass mechanisms that accept arbitrary caller tokens without verifying the trusted `statix_csrf` cookie.
- **Resource Exhaustion & DoS Prevention:**
  - Password hashing parameters (`argon2id`) must enforce strict upper bounds (`memory <= 256MB`, `iterations <= 10`, `parallelism <= 16`) to prevent memory/CPU exhaustion DoS attacks.
  - Rate limiting trackers must employ proactive cleanup/eviction to prevent unbounded memory growth from failed attempts.
- **Input Validation:** Validate all network endpoints and listen addresses (valid `host:port` format and ports between 1 and 65535).

## 6. Required Validation & Evidence Before Completion

Run checks proportional to the change, and run the full relevant set before declaring completion:

```powershell
# 1. Static Analysis & Linting
go vet ./...
golangci-lint run ./...

# 2. Comprehensive Test Suite
go test -v -count=1 ./...

# 3. Race Detector (mandatory for concurrency changes; executed in CI on Linux/macOS)
go test -v -race -count=1 ./...

# 4. Coverage Gate
go test -cover ./...

# 5. Parser Fuzzing (for /proc parsers)
go test -fuzz=FuzzProcStat -fuzztime=5s ./internal/metrics
go test -fuzz=FuzzMemInfo -fuzztime=5s ./internal/metrics
go test -fuzz=FuzzNetDev -fuzztime=5s ./internal/metrics

# 6. Performance Benchmarks
go test -bench="." -benchmem ./internal/metrics

# 7. Compilation Verification
go build ./...
```

- Never bypass, disable, or delete a failing test or linter rule without explicit user authorization.
- Add the smallest runnable regression test for every bugfix, security patch, or parsing boundary.

## 7. Release and Supply-Chain Security

- Release builds are strictly triggered on immutable Git tags matching `v*`.
- GitHub Actions workflows must use least-privilege token permissions (`contents: write`).
- Binaries are cross-compiled for Linux `amd64` and `arm64` with stripped symbols (`-ldflags="-s -w"`).
- Every release binary must generate and publish an accompanying `.sha256` checksum file.
- Never overwrite existing release tags or commit history.

## 8. Delegation and Completion

- Subagents must receive a bounded, non-overlapping task and must read this file first.
- Subagents must report changed files, commands executed, verification results, and any assumptions made.
- A task is complete only when the requested behavior is implemented, verification passes with observable evidence, and the Git diff is reviewed.

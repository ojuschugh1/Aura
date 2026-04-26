# Aura

[![CI](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml/badge.svg)](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml)
[![Release](https://github.com/ojuschugh1/Aura/actions/workflows/release.yml/badge.svg)](https://github.com/ojuschugh1/Aura/releases/latest)

**Your AI remembers what it did, and proves it.**

Aura is a local-first daemon that gives every AI tool you use — Claude Code, Cursor, Kiro, Gemini CLI — persistent memory, claim verification, token compression, dependency scanning, and a compounding knowledge wiki. One binary. Zero cloud. Works across tools.

**Current status: v0.9-dev** — 20 packages, ~18,000 lines of Go, 450+ passing tests.

---

## The problem

You use Claude Code in the morning, switch to Cursor after lunch, and ask ChatGPT a quick question at night. Every tool starts from zero. Your decisions, your context, your reasoning — gone at the session boundary.

And when the AI says "I created the file and installed the package" — did it actually? You have no way to know without checking yourself.

And the knowledge you build up — decisions, research, context — it's scattered across chat histories that disappear. Nothing compounds. Every session starts from scratch.

Aura fixes all three.

## What it does

**Cross-tool memory** — Store decisions, context, and project state in one place. Any MCP-compatible tool reads from the same memory.

```
aura memory add "auth" "JWT tokens, 24h expiry, refresh via httpOnly cookie"
aura memory add "stack" "Go backend, React frontend, PostgreSQL"
aura memory ls
```

**Claim verification** — Parse what the AI said it did. Check it against the real filesystem and git history. Get a truth score.

```
aura verify
# Truth score: 83%
# [PASS] created src/auth.ts
# [FAIL] installed jsonwebtoken — not found in lockfile
# [PASS] modified config.toml
```

**Token compression** — Pipe context through sqz before it hits the model. Save tokens, save money.

```
aura compact
# original:   2,400 tokens
# compressed: 1,800 tokens
# reduction:  25%
```

**Dependency scanning** — Catch phantom and unused dependencies in agent-written code before they ship.

```
aura scan
# [phantom] axios at src/api.js:3 (confidence: 1.00)
# [unused] lodash in package.json (confidence: 1.00)
# total: 4 findings (4 high-risk)
```

**Cost tracking** — See what your AI sessions actually cost.

```
aura cost --daily
# input:  45,000 tokens
# output: 12,000 tokens
# cost:   $0.23
# saved:  $0.08 (12,000 tokens compressed)
```

**Action escrow** — Intercept destructive agent actions and require your approval before execution.

```
aura trust --duration 15         # auto-approve for 15 minutes
aura trust --path ./src/test     # auto-approve writes to test dir only
```

**Doom loop detection** — Detects when an AI agent repeats the same failed action 3+ times and alerts you.

**Model routing** — Route tasks to the right model based on complexity. Use cheap models for simple tasks, capable models for hard ones. Budget limits stop runaway spending.

**Session traces** — Record full agent sessions as replayable traces. Search, export, and replay them.

```
aura trace last                  # last session summary
aura trace search "deploy"       # search across all traces
aura replay <session_id>         # replay and diff
```

**Auto-capture** — Automatically extracts decisions from AI sessions ("we decided to use PostgreSQL", "going with microservices") and stores them in memory without manual effort.

**Knowledge wiki** — A persistent, compounding knowledge base maintained by your AI tools. Ingest sources once — files, URLs, or tool output — and Aura builds interlinked pages that get richer with every source you add and every question you ask. Navigate the knowledge graph, trace connections between concepts, visualize the whole map in your browser, and let Aura watch your docs folder for automatic updates. Zero LLM calls. Inspired by [Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).

```
aura wiki ingest design.md              # ingest a file
aura wiki ingest https://example.com    # ingest a URL (HTML → markdown)
aura wiki ingest --dir ./docs           # batch ingest a whole folder

aura wiki query "authentication"        # search and synthesise
aura wiki query "auth" --save           # file the answer as a synthesis page

aura wiki trace auth-service database   # shortest path between two pages
# auth-service → jwt-tokens → database (2 hops)

aura wiki nearby postgresql --depth 2   # neighborhood exploration
# 5 page(s) near postgresql (depth 2):
# redis            entity    1 linked_from
# database-layer   concept   1 links_to
# ...

aura wiki context auth-service          # full 360° view
# confidence: 85% (strong)
# links to (3): jwt-tokens, database, session-mgmt
# linked from (2): api-gateway, user-service
# backed by (2 sources): #1 design.md, #3 auth-rfc.md

aura wiki viz                           # interactive HTML knowledge map
# knowledge map: ~/.aura/wiki-map.html (42 nodes, 87 edges)
# open in any browser to explore

aura wiki watch ./docs                  # auto-ingest on file changes
# [auto] meeting-notes.md → 3 created, 1 updated

aura wiki lint                          # health-check with suggestions
# health:  87%
# suggestions (4):
#   [create_page] Create "redis-caching" — referenced but missing
#   [split_page] "architecture" has 620 words — consider splitting
#   [investigate] "old-api" hasn't been updated in 30+ days

aura wiki schema --format claude        # generate CLAUDE.md for your LLM
aura wiki filter "category=entity AND link_count>3"  # Dataview-style queries
aura wiki export                        # Obsidian-compatible markdown + YAML frontmatter
aura wiki graph                         # connectivity stats: hubs, clusters, density
aura wiki feed scan.json --tool ghostdep  # feed tool output into wiki
```

## Install

### Build from source (recommended for development)

```bash
git clone https://github.com/ojuschugh1/Aura.git
cd Aura
go build -o aura ./cmd/aura/
```

### Using go install

```bash
go install github.com/ojuschugh1/Aura/cmd/aura@latest
```

### Using the install script

```bash
curl -fsSL https://raw.githubusercontent.com/ojuschugh1/Aura/main/install.sh | sh
```

### Using the Makefile

```bash
make build        # builds to bin/aura
make test         # runs all tests
make release      # cross-compile for all platforms
```

## Quick start

```bash
# build the binary
go build -o aura ./cmd/aura/

# start the daemon
./aura init

# store some context
./aura memory add "architecture" "event sourcing, not CRUD"
./aura memory add "db" "PostgreSQL with pgvector"

# check it persists
./aura memory ls

# JSON output for scripting
./aura memory ls --json

# verify what your AI actually did
./aura verify

# scan for bad dependencies
./aura scan

# see your costs
./aura cost

# check daemon status
./aura status

# stop the daemon
./aura stop

# build a knowledge wiki
./aura wiki ingest README.md
./aura wiki query "architecture"
./aura wiki ls
./aura wiki lint
./aura wiki viz                          # open wiki-map.html in browser
./aura wiki schema --format claude       # generate CLAUDE.md for your LLM
```

## Testing

### Run all tests

```bash
go test ./...
```

### Run tests with verbose output

```bash
go test -v ./...
```

### Run tests for a specific package

```bash
go test -v ./internal/memory/...     # memory store tests
go test -v ./internal/db/...         # database tests
go test -v ./internal/daemon/...     # daemon lifecycle tests
go test -v ./internal/mcp/...        # MCP server tests
go test -v ./internal/verify/...     # claim verification tests
go test -v ./internal/compress/...   # compression tests
go test -v ./internal/cost/...       # cost tracking tests
go test -v ./internal/doomloop/...   # doom loop detection tests
go test -v ./internal/escrow/...     # action escrow tests
go test -v ./internal/policy/...     # policy engine tests
go test -v ./internal/scan/...       # dependency scanning tests
go test -v ./internal/trace/...      # trace recording tests
go test -v ./internal/multiagent/... # multi-agent memory tests
go test -v ./internal/router/...     # model router tests
go test -v ./internal/session/...    # session manager tests
go test -v ./internal/subprocess/... # binary resolution tests
go test -v ./internal/autocapture/...# auto-capture tests
go test -v ./internal/wiki/...       # wiki knowledge base tests
go test -v ./internal/cli/...        # CLI command tests
```

### Run tests with race detection

```bash
go test -race ./...
```

### Testing in Kiro

You can run tests directly from Kiro's terminal:

1. Open the terminal in Kiro (`` Ctrl+` `` or `` Cmd+` ``)
2. Run `go test ./...` to execute the full test suite
3. Run `go test -v ./internal/memory/...` to test a specific package
4. Run `go build -o aura ./cmd/aura/` to build the binary
5. Run `./aura --help` to see all available commands
6. Run `./aura init` to start the daemon and test it interactively

You can also test individual CLI commands without starting the daemon — the memory commands work standalone since they open the SQLite database directly:

```bash
# Build and test memory commands (no daemon needed)
go build -o aura ./cmd/aura/
./aura memory add "test.key" "test-value" --dir /tmp/aura-test
./aura memory get "test.key" --dir /tmp/aura-test
./aura memory ls --dir /tmp/aura-test
./aura memory ls --json --dir /tmp/aura-test
./aura memory rm "test.key" --dir /tmp/aura-test

# Test version and help
./aura version
./aura version --json
./aura --help
./aura memory --help

# Generate MCP config snippets
./aura setup claude
./aura setup cursor
./aura setup kiro

# Shell completions
./aura completion bash
./aura completion zsh
./aura completion fish

# Wiki commands (no daemon needed)
./aura wiki ingest README.md --dir /tmp/aura-test
./aura wiki query "architecture" --dir /tmp/aura-test
./aura wiki ls --dir /tmp/aura-test
./aura wiki lint --dir /tmp/aura-test
./aura wiki trace <slug-a> <slug-b> --dir /tmp/aura-test
./aura wiki nearby <slug> --dir /tmp/aura-test
./aura wiki context <slug> --dir /tmp/aura-test
./aura wiki viz --dir /tmp/aura-test
./aura wiki schema --format claude --dir /tmp/aura-test
./aura wiki filter "category=entity" --dir /tmp/aura-test
./aura wiki log --dir /tmp/aura-test
./aura wiki index --dir /tmp/aura-test
./aura wiki sources --dir /tmp/aura-test
```

## Connect to your AI tools

```bash
# generate MCP config for your tool
aura setup claude    # Claude Code
aura setup cursor    # Cursor
aura setup kiro      # Kiro
```

This prints the JSON snippet you need to add to your tool's MCP config file.

## How it works

Aura runs as a local Go daemon with an embedded MCP server and SQLite database. It integrates with three Rust tools as optional subprocesses:

- **sqz** — token compression (auto-downloaded on first use)
- **claimcheck** — claim verification (auto-downloaded on first use)
- **ghostdep** — dependency scanning (auto-downloaded on first use)

The core memory, MCP server, cost tracking, policy engine, doom loop detection, session traces, and model router work without any of these — they're pure Go with zero external dependencies.

```
┌─────────────────────────────────────────┐
│           Your AI Tools                 │
│  Claude Code · Cursor · Kiro · Gemini  │
└──────────────┬──────────────────────────┘
               │ MCP Protocol
┌──────────────▼──────────────────────────┐
│           Aura Daemon                   │
│                                         │
│  Memory Store ─── SQLite (WAL mode)     │
│  MCP Server ───── localhost:7437        │
│  Session Manager                        │
│  Claim Verifier ─ claimcheck (Rust)     │
│  Compressor ───── sqz (Rust)            │
│  Dep Scanner ──── ghostdep (Rust)       │
│  Cost Tracker                           │
│  Policy Engine ── .aura/policy.toml     │
│  Action Escrow                          │
│  Doom Loop Detector                     │
│  Auto-Capture Engine                    │
│  Trace Recorder                         │
│  Model Router ─── .aura/routing.toml    │
│  Wiki Engine ──── knowledge base        │
│  Knowledge Map ── interactive HTML      │
│  File Watcher ─── auto-ingest           │
│  Multi-Agent Coordinator                │
└─────────────────────────────────────────┘
```

All data stays on your machine in `~/.aura/`. Nothing leaves your system unless you explicitly configure cloud sync.

### Configuration files

On `aura init`, three config files are generated in your `.aura/` directory:

| File | Purpose |
|------|---------|
| `config.toml` | Daemon port, log level, auth secret, memory limits, compression settings, model pricing, trace TTL |
| `policy.toml` | Action approval rules — which actions are auto-approved, require approval, or denied |
| `routing.toml` | Model routing configuration — which models handle which complexity levels |

The policy engine supports hot-reload — edit `policy.toml` while the daemon is running and changes take effect within 5 seconds.

## All commands

```
aura init                        # start daemon, generate config files
aura init --install-deps         # start daemon and download all Rust binaries
aura init --skip-deps            # start daemon, skip dependency check
aura status                      # show daemon state
aura stop                        # stop daemon

aura memory add <key> <value>    # store context
aura memory get <key>            # retrieve context
aura memory ls                   # list all entries
aura memory ls --agent <tool>    # filter by source tool
aura memory ls --auto            # show only auto-captured entries
aura memory rm <key>             # delete entry
aura memory export               # export to JSON
aura memory import               # import from JSON

aura verify                      # verify agent claims (current session)
aura verify --session <id>       # verify specific session
aura compact                     # compress context
aura scan                        # scan for phantom dependencies
aura scan --sarif                # SARIF output for CI/CD
aura scan --fix                  # suggest auto-fixes

aura cost                        # current session cost
aura cost --daily                # daily breakdown
aura cost --weekly               # weekly breakdown

aura trace last                  # last session trace
aura trace show <id>             # full trace
aura trace search <query>        # search traces
aura trace export <id>           # export trace (JSON/HTML)
aura trace pin <id>              # pin trace (prevent pruning)
aura replay <session_id>         # replay and diff

aura trust --duration 15         # auto-approve for 15 minutes
aura trust --path ./src/test     # auto-approve writes to test dir

aura wiki ingest <file>          # ingest a source into the wiki
aura wiki ingest <url>           # ingest a URL (HTML → markdown)
aura wiki ingest <text> --title  # ingest inline text
aura wiki ingest --dir <folder>  # batch ingest all files in a directory
aura wiki query <terms>          # search wiki and synthesise answer
aura wiki query <terms> --save   # search and file the answer as a wiki page
aura wiki lint                   # health-check with actionable suggestions
aura wiki ls                     # list all wiki pages
aura wiki ls --category entity   # filter by category
aura wiki show <slug>            # show full page content
aura wiki search <query>         # search pages by title/content
aura wiki trace <from> <to>      # shortest path between two pages
aura wiki nearby <slug>          # pages within N hops (--depth 2)
aura wiki context <slug>         # full 360° view: links, sources, confidence
aura wiki viz                    # interactive HTML knowledge map
aura wiki viz --out <file>       # export map to specific file
aura wiki watch <dir>            # auto-ingest on file changes
aura wiki schema                 # generate LLM schema (CLAUDE.md / AGENTS.md)
aura wiki schema --format claude # target a specific LLM tool
aura wiki filter <expression>    # Dataview-style metadata queries
aura wiki graph                  # connectivity stats: hubs, clusters, density
aura wiki log                    # show wiki activity log
aura wiki index                  # show full wiki catalog
aura wiki sources                # list all ingested raw sources
aura wiki export                 # Obsidian-compatible markdown + YAML frontmatter
aura wiki export --out <dir>     # export to a specific directory
aura wiki rm <slug>              # delete a wiki page

aura wiki feed <file> --tool sqz       # feed sqz compression stats into wiki
aura wiki feed <file> --tool ghostdep  # feed dependency scan results
aura wiki feed <file> --tool claimcheck # feed verification reports
aura wiki feed <file> --tool etch      # feed API change detection
aura wiki feed <file> --tool <name>    # feed any tool's JSON output

aura setup <tool>                # generate MCP config (claude/cursor/kiro)
aura version                     # version info
aura completion <shell>          # shell completions (bash/zsh/fish)
```

All commands support `--json` for machine-readable output and `--dir` to override the data directory.

## Project structure

```
aura/
├── cmd/aura/main.go              # CLI entry point (cobra)
├── internal/
│   ├── autocapture/               # decision extraction from transcripts
│   ├── cli/                       # all CLI command implementations
│   ├── compress/                  # token compression (sqz subprocess)
│   ├── cost/                      # cost tracking and reporting
│   ├── daemon/                    # daemon lifecycle, config, logging
│   ├── db/                        # SQLite connection, migrations
│   ├── doomloop/                  # stuck-agent detection
│   ├── escrow/                    # action escrow and trust windows
│   ├── mcp/                       # MCP server (HTTP/JSON-RPC)
│   ├── memory/                    # persistent key-value memory store
│   ├── multiagent/                # shared memory coordination
│   ├── policy/                    # configurable action approval rules
│   ├── router/                    # model routing and budget control
│   ├── scan/                      # dependency scanning (ghostdep)
│   ├── session/                   # session lifecycle management
│   ├── subprocess/                # Rust binary resolution and download
│   ├── trace/                     # session trace recording and replay
│   ├── verify/                    # claim verification (claimcheck)
│   └── wiki/                      # LLM-maintained knowledge base
├── pkg/types/                     # shared Go types
├── Makefile                       # build, test, release targets
├── install.sh                     # curl-pipe installer
├── go.mod
└── go.sum
```

## Roadmap

- [x] v0.1 — Daemon, memory, MCP server, claim verification, data integrity, CLI, install
- [x] v0.2 — Auto-capture from sessions, token compression, cost tracking, doom loop detection
- [x] v0.3 — Action escrow, policy engine, dependency scanning
- [x] v0.4 — Session trace recording and replay
- [x] v0.5 — Multi-agent shared memory
- [x] v0.6 — Model router with budget control
- [x] v0.7 — LLM Wiki knowledge base (ingest, query, lint, index)
- [x] v0.8 — Wiki advanced features (traversal, confidence, visualization, watch, schema, URL ingest, filters, tool pipeline)
- [ ] v0.9 — Desktop app (Tauri)
- [ ] v0.10 — Enterprise features (team sync, SSO, audit logs)
- [ ] v0.11 — Browser extension (Chrome/Firefox)
- [ ] v1.0 — Plugin system and public API

## Tech stack

| Layer | Technology | Why |
|-------|-----------|-----|
| Core daemon | Go 1.22+ | Single static binary, excellent concurrency |
| Storage | SQLite via modernc.org/sqlite | Embedded, crash-safe WAL mode, pure Go (no CGO) |
| CLI | cobra + viper | Standard Go CLI tooling |
| Configuration | TOML | Human-readable, well-supported |
| Compression | Rust (sqz binary) | Existing tool, called as subprocess |
| Verification | Rust (claimcheck binary) | Existing tool, called as subprocess |
| Scanning | Rust (ghostdep binary) | Tree-sitter AST analysis |

## Why not just use MemPalace / Mem0 / Engram?

| | Aura | MemPalace | Mem0 | Engram |
|---|---|---|---|---|
| Cross-tool memory | ✅ | ❌ | ❌ | ✅ |
| Claim verification | ✅ | ❌ | ❌ | ❌ |
| Token compression | ✅ | ✅ | ❌ | ❌ |
| Dependency scanning | ✅ | ❌ | ❌ | ❌ |
| Cost tracking | ✅ | ❌ | ❌ | ❌ |
| Action escrow | ✅ | ❌ | ❌ | ❌ |
| Doom loop detection | ✅ | ❌ | ❌ | ❌ |
| Model routing | ✅ | ❌ | ❌ | ❌ |
| Session traces | ✅ | ❌ | ❌ | ❌ |
| Auto-capture | ✅ | ❌ | ❌ | ❌ |
| Knowledge wiki | ✅ | ❌ | ❌ | ❌ |
| Knowledge map (HTML) | ✅ | ❌ | ❌ | ❌ |
| Graph traversal | ✅ | ❌ | ❌ | ❌ |
| Confidence scoring | ✅ | ❌ | ❌ | ❌ |
| File watch auto-ingest | ✅ | ❌ | ❌ | ❌ |
| URL ingestion | ✅ | ❌ | ❌ | ❌ |
| LLM schema generator | ✅ | ❌ | ❌ | ❌ |
| Single binary (Go) | ✅ | ❌ (Python) | ❌ (Python) | ✅ |
| Local-first | ✅ | ✅ | ❌ | ✅ |

## Contributing

Found a bug? Have an idea? Open an issue. PRs welcome.

## License

MIT — see [LICENSE](LICENSE).

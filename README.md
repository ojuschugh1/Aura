# Aura

**Your AI remembers what it did, and proves it.**

Aura is a local-first daemon that gives every AI tool you use — Claude Code, Cursor, Kiro, Gemini CLI — persistent memory, claim verification, token compression, and dependency scanning. One binary. Zero cloud. Works across tools.

---

## The problem

You use Claude Code in the morning, switch to Cursor after lunch, and ask ChatGPT a quick question at night. Every tool starts from zero. Your decisions, your context, your reasoning — gone at the session boundary.

And when the AI says "I created the file and installed the package" — did it actually? You have no way to know without checking yourself.

Aura fixes both.

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
cat large-context.txt | aura compact
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

## Install

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/ojuschugh1/Aura/main/install.sh | sh

# or build from source
go install github.com/ojuschugh1/Aura/cmd/aura@latest

# or clone and build
git clone https://github.com/ojuschugh1/Aura.git
cd Aura
go build -o aura ./cmd/aura/
```

## Quick start

```bash
# start the daemon
aura init

# store some context
aura memory add "architecture" "event sourcing, not CRUD"
aura memory add "db" "PostgreSQL with pgvector"

# check it persists
aura memory ls

# verify what your AI actually did
aura verify

# scan for bad dependencies
aura scan

# see your costs
aura cost
```

## Connect to your AI tools

```bash
# generate MCP config for your tool
aura setup claude    # Claude Code
aura setup cursor    # Cursor
aura setup kiro      # Kiro
```

This prints the JSON snippet you need to add to your tool's MCP config. Two lines, and your AI tool can read/write Aura memory.

## How it works

Aura runs as a local Go daemon with an embedded MCP server and SQLite database. It integrates with three Rust tools as optional subprocesses:

- **sqz** — token compression (auto-downloaded if missing)
- **claimcheck** — claim verification (auto-downloaded if missing)
- **ghostdep** — dependency scanning (auto-downloaded if missing)

The core memory and MCP server work without any of these — they're pure Go with zero external dependencies.

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
│  Claim Verifier ─ claimcheck (Rust)     │
│  Compressor ───── sqz (Rust)            │
│  Dep Scanner ──── ghostdep (Rust)       │
│  Cost Tracker                           │
│  Policy Engine                          │
│  Doom Loop Detector                     │
└─────────────────────────────────────────┘
```

All data stays on your machine in `~/.aura/`. Nothing leaves your system unless you explicitly configure cloud sync.

## All commands

```
aura init                        # start daemon
aura status                      # show daemon state
aura stop                        # stop daemon

aura memory add <key> <value>    # store context
aura memory get <key>            # retrieve context
aura memory ls                   # list all entries
aura memory rm <key>             # delete entry
aura memory export               # export to JSON
aura memory import               # import from JSON

aura verify                      # verify agent claims
aura compact                     # compress context (pipe from stdin)
aura scan                        # scan for bad dependencies
aura scan --sarif                # SARIF output for CI/CD

aura cost                        # current session cost
aura cost --daily                # daily breakdown
aura cost --weekly               # weekly breakdown

aura trace last                  # last session trace
aura trace show <id>             # full trace
aura trace search <query>        # search traces

aura trust --duration 15         # auto-approve for 15 minutes
aura trust --path ./src/test     # auto-approve writes to test dir

aura setup <tool>                # generate MCP config
aura version                     # version info
aura completion <shell>          # shell completions (bash/zsh/fish)
```

All commands support `--json` for machine-readable output.

## Roadmap

- [x] v0.1 — Memory, MCP server, verification, compression, scanning
- [ ] v0.2 — Auto-capture from sessions, cost tracking, doom loop detection
- [ ] v0.3 — Action escrow, policy engine, blast radius control
- [ ] v0.4 — Session trace recording and replay
- [ ] v0.5 — Multi-agent shared memory
- [ ] v0.6 — Model router with budget control
- [ ] v0.7 — Desktop app (Tauri)
- [ ] v0.8 — Enterprise features (team sync, SSO, audit logs)
- [ ] v0.9 — Browser extension (Chrome/Firefox)
- [ ] v1.0 — Plugin system and public API

## Tech stack

- **Go** — single static binary, zero runtime dependencies
- **SQLite** (via modernc.org/sqlite) — embedded, crash-safe, WAL mode
- **MCP** — Model Context Protocol for universal AI tool integration
- **cobra** — CLI framework
- **sqz** (Rust) — token compression (optional, auto-downloaded)
- **claimcheck** (Rust) — claim verification (optional, auto-downloaded)
- **ghostdep** (Rust) — dependency scanning (optional, auto-downloaded)

## Why not just use MemPalace / Mem0 / Engram?

| | Aura | MemPalace | Mem0 | Engram |
|---|---|---|---|---|
| Cross-tool memory | ✅ | ❌ | ❌ | ✅ |
| Claim verification | ✅ | ❌ | ❌ | ❌ |
| Token compression | ✅ | ✅ | ❌ | ❌ |
| Dependency scanning | ✅ | ❌ | ❌ | ❌ |
| Cost tracking | ✅ | ❌ | ❌ | ❌ |
| Action escrow | ✅ | ❌ | ❌ | ❌ |
| Single binary (Go) | ✅ | ❌ (Python) | ❌ (Python) | ✅ |
| Local-first | ✅ | ✅ | ❌ | ✅ |

## Contributing

Found a bug? Have an idea? Open an issue. PRs welcome.

## License

MIT — see [LICENSE](LICENSE).

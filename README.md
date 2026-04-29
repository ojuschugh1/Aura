# Aura

[![CI](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml/badge.svg)](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml)
[![Release](https://github.com/ojuschugh1/Aura/actions/workflows/release.yml/badge.svg)](https://github.com/ojuschugh1/Aura/releases/latest)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**Your AI remembers what it did, proves it, and gets smarter every session.**

Aura is a local-first daemon that gives every AI tool you use — Claude Code, Cursor, Kiro, Gemini CLI — persistent memory, claim verification, MCP traffic observability, OWASP compliance scoring, and a self-improving knowledge wiki. One binary. Zero cloud. Works across all tools.

**[📖 Full Documentation](https://ojuschugh1.github.io/Aura)** · **[Releases](https://github.com/ojuschugh1/Aura/releases)** · **[Issues](https://github.com/ojuschugh1/Aura/issues)**

**Current status: v1.0-dev** — 23 packages, ~25,000 lines of Go, 490+ passing tests.

---

## The problem

You use Claude Code in the morning, switch to Cursor after lunch, and ask ChatGPT a quick question at night. Every tool starts from zero. Your decisions, your context, your reasoning — gone at the session boundary.

When the AI says "I created the file and installed the package" — did it actually? You have no way to know without checking yourself.

The knowledge you build up — decisions, research, architecture — is scattered across chat histories that disappear. Nothing compounds. Every session starts from scratch.

**Aura fixes all three.**

---

## Install

```bash
# One-line install (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/ojuschugh1/Aura/main/install.sh | sh

# Or with go install
go install github.com/ojuschugh1/Aura/cmd/aura@latest

# Or build from source
git clone https://github.com/ojuschugh1/Aura.git && cd Aura && go build -o aura ./cmd/aura/
```

Then start the daemon — Aura begins learning immediately:

```bash
aura init
```

---

## What it does

### Claim verification

The most novel thing Aura does. When your AI says "I created the file and installed the package" — Aura checks whether that's actually true.

```bash
aura verify
# Truth score: 83%
# [PASS] created src/auth.ts       — file exists
# [FAIL] installed jsonwebtoken    — not found in lockfile
# [PASS] modified config.toml      — found in git diff
```

### Cross-tool memory

Store decisions and context once. Every AI tool reads from the same store.

```bash
aura memory add "auth"  "JWT tokens, 24h expiry, refresh via httpOnly cookie"
aura memory add "stack" "Go backend, React frontend, PostgreSQL"
aura memory ls
```

### Token compression

Pipe context through sqz before it hits the model. Save tokens, save money.

```bash
cat context.txt | aura compact
# original:   2,400 tokens
# compressed: 1,800 tokens
# reduction:  25%
```

### Dependency scanning

Catch phantom and unused dependencies in agent-written code before they ship.

```bash
aura scan
# [phantom] axios at src/api.js:3 (confidence: 1.00)
# [unused]  lodash in package.json (confidence: 1.00)
# total: 4 findings (4 high-risk)
```

### Cost tracking

See what your AI sessions actually cost.

```bash
aura cost --daily
# input:  45,000 tokens  output: 12,000 tokens
# cost:   $0.23          saved:  $0.08 (12,000 tokens compressed)
```

### Action escrow

Intercept destructive agent actions and require your approval before execution.

```bash
aura trust --duration 15        # auto-approve for 15 minutes
aura trust --path ./src/test    # auto-approve writes to test dir only
```

### Self-improving knowledge wiki

A persistent, compounding knowledge base that **automatically learns from every session** — no manual steps, no IDE plugins. Just run `aura init` and it starts learning.

```bash
# Ingest sources
aura wiki ingest design.md                    # from a file
aura wiki ingest https://example.com/article  # from a URL
aura wiki ingest --batch ./docs               # entire folder

# Query and save answers
aura wiki query "authentication"              # search and synthesise
aura wiki query "auth" --save                 # file the answer as a page

# Navigate the knowledge graph
aura wiki trace auth-service database         # shortest path between pages
aura wiki nearby postgresql --depth 2         # neighborhood exploration
aura wiki context auth-service                # full 360° view with confidence

# Visualize
aura wiki viz                                 # interactive HTML knowledge map

# Health and lifecycle
aura wiki lint                                # health-check with suggestions
aura wiki metabolize                          # decay, consolidate, pressure check

# Security
aura wiki access <slug> team                  # set page visibility
aura wiki verify-chain                        # tamper detection on audit trail

# Generate LLM schema
aura wiki schema --format claude > CLAUDE.md  # teach Claude how to use the wiki
aura wiki schema --format kiro                # Kiro steering file
```

**Auto-learning** — the daemon automatically:
- Captures decisions from every MCP call in real-time (no manual `memory add` needed)
- Ingests session transcripts when sessions end
- Promotes important memory entries to wiki pages every 5 minutes
- Runs knowledge metabolism every 6 hours (decay, contradictions, consolidation)

### MCP Proxy — the Wireshark for AI agents

Sit between your AI tools and their MCP servers. See everything, verify it, stop it.

```bash
# Start the proxy
aura proxy start --upstream http://localhost:7437/mcp --port 7438

# Point your AI tool at the proxy instead of the real server
# In MCP config: "url": "http://localhost:7438/proxy/default/mcp"

# See what your agent is doing
curl http://localhost:7438/proxy/stats
curl http://localhost:7438/proxy/log
```

**OWASP Agentic Top 10 compliance** — every proxied call is scored against the OWASP Agentic Top 10 risks: excessive agency, tool misuse, memory poisoning, identity abuse, resource exhaustion.

```bash
aura proxy owasp
# ASI01  Excessive Agency       Agent calling destructive tools without approval
# ASI03  Tool Misuse            Shell injection or parameter manipulation
# ASI05  Memory Poisoning       Writing contradictory or malicious memory
# ...
```

**Context cliff protection** — tracks token usage per session. Warns at 75%, critical at 90%. Prevents the silent degradation that happens when agents hit their context limit.

**Session replay** — generates diff reports showing what the agent actually did vs what it claimed. Tool breakdown, file changes, claim verification, full timeline.

### A2A Memory Bridge

Implements Google's Agent-to-Agent protocol. Other agents can discover Aura and share verified memory:

```bash
# Agent discovery
curl http://localhost:7439/.well-known/agent.json

# Agent A (Claude) stores a decision → Agent B (Cursor) reads it
curl -X POST http://localhost:7439/a2a/memory \
  -d '{"action":"write","key":"db","value":"PostgreSQL","agent_id":"claude"}'
```

---

## How it works

```
┌─────────────────────────────────────────┐
│           Your AI Tools                 │
│  Claude Code · Cursor · Kiro · Gemini  │
└──────────────┬──────────────────────────┘
               │ MCP Protocol
┌──────────────▼──────────────────────────┐
│         Aura MCP Proxy (:7438)          │
│  Traffic logging · OWASP scoring        │
│  Context cliff · Policy enforcement     │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│           Aura Daemon (:7437)           │
│                                         │
│  Memory Store ─── SQLite (WAL mode)     │
│  MCP Server ───── HTTP/JSON-RPC         │
│  Session Manager                        │
│  Claim Verifier ─ claimcheck (Rust)     │
│  Compressor ───── sqz (Rust)            │
│  Dep Scanner ──── ghostdep (Rust)       │
│  Cost Tracker                           │
│  Policy Engine ── .aura/policy.toml     │
│  Action Escrow                          │
│  Doom Loop Detector                     │
│  Auto-Capture ─── real-time from MCP    │
│  Trace Recorder                         │
│  Model Router ─── .aura/routing.toml    │
│  Wiki Engine ──── knowledge base        │
│  Auto-Learner ─── self-improving        │
│  Audit Chain ──── tamper-evident log    │
│  Multi-Agent Coordinator                │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│         A2A Bridge (:7439)              │
│  Agent discovery · Shared memory        │
│  Cross-agent collaboration              │
└─────────────────────────────────────────┘
```

All data stays on your machine in `~/.aura/`. Nothing leaves your system.

**Rust tool dependencies** — sqz (compression), claimcheck (verification), and ghostdep (scanning) are optional Rust binaries. Aura's core features (memory, wiki, cost tracking, session traces, policy engine) work without them. If you have them installed via `cargo install`, Aura finds them automatically. The `--install-deps` flag downloads pre-built binaries, but this requires the Rust tools to have published GitHub Releases — check each tool's repo for availability.

---

## Connect to your AI tools

```bash
aura setup claude    # Claude Code  → ~/.claude/settings.json
aura setup cursor    # Cursor       → ~/.cursor/mcp.json
aura setup kiro      # Kiro         → .kiro/settings/mcp.json
```

---

## All commands

```
# Daemon
aura init                        # start daemon (begins auto-learning)
aura init --install-deps         # also download Rust binaries
aura status                      # daemon state, memory count, session
aura stop                        # graceful shutdown

# Memory
aura memory add <key> <value>    # store context
aura memory get <key>            # retrieve
aura memory ls [--agent] [--auto] # list, filter
aura memory rm <key>             # delete
aura memory export / import      # JSON backup

# Verification & tools
aura verify [--session <id>]     # verify agent claims
aura compact                     # compress context (stdin)
aura scan [--sarif] [--fix]      # phantom dependency scan
aura cost [--daily] [--weekly]   # token cost report
aura trust [--duration] [--path] # action escrow windows

# Session traces
aura trace last / show / search / export / pin
aura replay <session_id>

# Wiki — ingest
aura wiki ingest <file|url|text> [--title] [--format]
aura wiki ingest --batch <dir>   # batch ingest directory

# Wiki — query & navigate
aura wiki query <terms> [--save] # search + optional save
aura wiki search <query>         # title/content search
aura wiki trace <from> <to>      # shortest path
aura wiki nearby <slug> [--depth N]  # neighborhood
aura wiki context <slug>         # 360° view: links, sources, confidence

# Wiki — browse
aura wiki ls [--category]        # list pages
aura wiki show <slug>            # full page content
aura wiki index                  # full catalog
aura wiki sources                # ingested raw sources
aura wiki log [--limit N]        # activity log

# Wiki — health & lifecycle
aura wiki lint                   # orphans, stale, contradictions, suggestions
aura wiki metabolize             # decay, consolidate, pressure check
aura wiki pressure [slug]        # contradiction pressure
aura wiki graph                  # connectivity stats

# Wiki — output
aura wiki viz [--out file]       # interactive HTML knowledge map
aura wiki export [--out dir]     # Obsidian-compatible markdown
aura wiki schema [--format]      # CLAUDE.md / AGENTS.md / Kiro steering
aura wiki filter <expression>    # metadata queries

# Wiki — security
aura wiki access <slug> <tier>   # set public/team/private
aura wiki audit [slug]           # immutable audit trail
aura wiki verify-chain           # tamper detection

# Wiki — tool pipeline
aura wiki feed <file> --tool sqz|ghostdep|claimcheck|etch|<name>

# Wiki — maintenance
aura wiki rm <slug>              # delete page
aura wiki watch <dir>            # auto-ingest on file changes

# Proxy — MCP traffic observability
aura proxy start [--port] [--upstream URL]  # start MCP proxy
aura proxy stats                 # traffic statistics
aura proxy log                   # recent proxied calls
aura proxy owasp                 # OWASP Agentic Top 10 report
aura proxy replay [session_id]   # session replay with diffs

# Setup
aura setup <claude|cursor|kiro>  # generate MCP config
aura version [--json]
aura completion <bash|zsh|fish>
```

All commands support `--json` for machine-readable output and `--dir` to override the data directory.

---

## Configuration

On `aura init`, three config files are generated in `~/.aura/`:

| File | Purpose |
|------|---------|
| `config.toml` | Port, log level, auth secret, memory limits, compression, pricing, trace TTL |
| `policy.toml` | Action approval rules — auto-approve, require-approval, deny |
| `routing.toml` | Model routing — which models handle which complexity levels |

The policy engine supports hot-reload — changes take effect within 5 seconds.

---

## Roadmap

- [x] v0.1 — Daemon, memory, MCP server, claim verification, CLI, install
- [x] v0.2 — Auto-capture, token compression, cost tracking, doom loop detection
- [x] v0.3 — Action escrow, policy engine, dependency scanning
- [x] v0.4 — Session trace recording and replay
- [x] v0.5 — Multi-agent shared memory
- [x] v0.6 — Model router with budget control
- [x] v0.7 — LLM Wiki (ingest, query, lint, index, log)
- [x] v0.8 — Wiki advanced (traversal, confidence, viz, watch, schema, URL, filters, tool pipeline)
- [x] v0.9 — Memory metabolism, audit chain, access tiers, auto-learner daemon
- [x] v0.10 — MCP proxy, OWASP scoring, context cliff, A2A bridge, session replay, real-time auto-capture
- [ ] v1.0 — Desktop app (Tauri), browser extension, plugin system

---

## Tech stack

| Layer | Technology |
|-------|-----------|
| Core | Go 1.22+, single static binary |
| Storage | SQLite via modernc.org/sqlite (pure Go, no CGO) |
| CLI | cobra + viper |
| Config | TOML |
| Compression | Rust (sqz) |
| Verification | Rust (claimcheck) |
| Scanning | Rust (ghostdep, tree-sitter) |

---

## Why not just use Claude's built-in memory?

Claude Projects, Cursor's context, and Copilot Workspace all have some form of memory. They work well within their own tool. The problem is they don't talk to each other.

If you use Claude Code in the morning and Cursor in the afternoon, Claude's memory doesn't transfer. If you switch tools next month, you start from zero. And none of them verify whether the AI actually did what it claimed.

Aura solves the **cross-tool** problem: one memory store, one wiki, one verification system that works with every MCP-compatible tool. It also does things no built-in memory does — claim verification, dependency scanning, cost tracking, knowledge metabolism.

If you only use one AI tool and never switch, built-in memory is fine. If you use multiple tools, or care about verification, Aura fills the gap.

---

## How Aura compares

Aura's unique strengths are cross-tool continuity, claim verification, and the self-improving wiki. Here's an honest comparison focused on what actually matters:

| Capability | Aura | Claude Projects | Cursor Memory | OpenMemory (Mem0) |
|-----------|------|----------------|---------------|-------------------|
| Cross-tool memory | ✅ MCP-native | ❌ Claude only | ❌ Cursor only | ✅ MCP-native |
| Claim verification | ✅ | ❌ | ❌ | ❌ |
| MCP proxy / observability | ✅ | ❌ | ❌ | ❌ |
| OWASP Agentic scoring | ✅ | ❌ | ❌ | ❌ |
| Context cliff protection | ✅ | ❌ | ❌ | ❌ |
| A2A protocol bridge | ✅ | ❌ | ❌ | ❌ |
| Knowledge wiki | ✅ compounding | ❌ | ❌ | ❌ |
| Real-time auto-capture | ✅ from MCP wire | ❌ | ❌ | ❌ |
| Token compression | ✅ via sqz | ❌ | ❌ | ❌ |
| Dependency scanning | ✅ via ghostdep | ❌ | ❌ | ❌ |
| Cost tracking | ✅ | ❌ | ❌ | ❌ |
| Local-first / private | ✅ SQLite on disk | ⚠️ cloud-stored | ⚠️ cloud-stored | ✅ self-hosted option |
| Single binary install | ✅ Go | N/A (built-in) | N/A (built-in) | ❌ Python |
| Immutable audit chain | ✅ tamper-evident | ❌ | ❌ | ❌ |
| Session replay with diff | ✅ | ❌ | ❌ | ❌ |

**Where others are better:** Claude Projects has deeper integration with Claude's context window. Cursor's memory is zero-config within Cursor. Mem0's cloud offering has team sync built in. Aura trades those conveniences for cross-tool portability and verification.

---

## Contributing

Found a bug? Have an idea? Open an issue. PRs welcome.

## License

MIT — see [LICENSE](LICENSE).

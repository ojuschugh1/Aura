---
layout: default
title: Command Reference
description: Complete reference for all Aura CLI commands.
---

# Command Reference

All commands support `--json` for machine-readable output and `--dir <path>` to override the data directory (default: `~/.aura`).

---

## Daemon

| Command | Description |
|---------|-------------|
| `aura init` | Start daemon, generate config files, begin auto-learning |
| `aura init --install-deps` | Also download Rust binaries (sqz, claimcheck, ghostdep) |
| `aura init --skip-deps` | Skip dependency check |
| `aura status` | Show daemon state, port, memory count, session ID |
| `aura stop` | Gracefully stop the daemon |

---

## Memory

| Command | Description |
|---------|-------------|
| `aura memory add <key> <value>` | Store a context entry |
| `aura memory get <key>` | Retrieve an entry by key |
| `aura memory ls` | List all entries |
| `aura memory ls --agent <tool>` | Filter by source tool |
| `aura memory ls --auto` | Show only auto-captured entries |
| `aura memory rm <key>` | Delete an entry |
| `aura memory export [--file]` | Export to JSON |
| `aura memory import [--file]` | Import from JSON |

---

## Verification & Tools

| Command | Description |
|---------|-------------|
| `aura verify` | Verify agent claims against filesystem and git |
| `aura verify --session <id>` | Verify a specific session |
| `aura compact` | Compress context via sqz (reads stdin) |
| `aura scan [path]` | Scan for phantom/unused dependencies |
| `aura scan --sarif` | SARIF output for CI/CD |
| `aura cost` | Current session cost |
| `aura cost --daily` | Daily cost breakdown |
| `aura cost --weekly` | Weekly cost breakdown |
| `aura trust --duration <min>` | Auto-approve actions for N minutes |
| `aura trust --path <dir>` | Auto-approve writes to a directory |

---

## Session Traces

| Command | Description |
|---------|-------------|
| `aura trace last` | Most recent trace summary |
| `aura trace show <id>` | Full trace content |
| `aura trace search <query>` | Search across all traces |
| `aura trace export <id> [--format json\|html]` | Export a trace |
| `aura trace pin <id>` | Pin trace (prevent pruning) |
| `aura replay <session_id>` | Replay trace and diff against current state |

---

## Wiki — Ingest

| Command | Description |
|---------|-------------|
| `aura wiki ingest <file>` | Ingest a file (auto-detects format) |
| `aura wiki ingest <url>` | Ingest a URL (HTML → markdown) |
| `aura wiki ingest --title <t> <text>` | Ingest inline text |
| `aura wiki ingest --batch <dir>` | Batch ingest all files in a directory |

---

## Wiki — Query & Navigate

| Command | Description |
|---------|-------------|
| `aura wiki query <terms>` | Search and synthesise answer |
| `aura wiki query <terms> --save` | Search and save answer as synthesis page |
| `aura wiki search <query>` | Search pages by title and content |
| `aura wiki trace <from> <to>` | Shortest path between two pages |
| `aura wiki nearby <slug> [--depth N]` | Pages within N hops |
| `aura wiki context <slug>` | Full 360° view: links, sources, confidence |

---

## Wiki — Browse

| Command | Description |
|---------|-------------|
| `aura wiki ls [--category]` | List all pages |
| `aura wiki show <slug>` | Full page content |
| `aura wiki index` | Full catalog with summaries |
| `aura wiki sources` | List all ingested raw sources |
| `aura wiki log [--limit N]` | Activity log |
| `aura wiki filter <expression>` | Metadata queries |

---

## Wiki — Health & Lifecycle

| Command | Description |
|---------|-------------|
| `aura wiki lint` | Health check: orphans, stale, contradictions, suggestions |
| `aura wiki metabolize` | Decay, consolidate, pressure check |
| `aura wiki pressure [slug]` | Show contradiction pressure |
| `aura wiki graph` | Connectivity stats: hubs, clusters, density |

---

## Wiki — Output

| Command | Description |
|---------|-------------|
| `aura wiki viz [--out file]` | Interactive HTML knowledge map |
| `aura wiki export [--out dir]` | Obsidian-compatible markdown with YAML frontmatter |
| `aura wiki schema [--format]` | Generate CLAUDE.md / AGENTS.md / Kiro steering |

Formats: `claude`, `cursor`, `kiro`, `codex`, `generic`

---

## Wiki — Security

| Command | Description |
|---------|-------------|
| `aura wiki access <slug> <tier>` | Set page access tier |
| `aura wiki audit [slug]` | Immutable audit trail |
| `aura wiki verify-chain` | Verify audit chain integrity |

Tiers: `public`, `team`, `private`

---

## Wiki — Tool Pipeline

```bash
aura wiki feed <file> --tool sqz         # SQZ compression stats
aura wiki feed <file> --tool ghostdep    # GhostDep scan results
aura wiki feed <file> --tool claimcheck  # ClaimCheck verification
aura wiki feed <file> --tool etch        # Etch API changes
aura wiki feed <file> --tool <name>      # Any JSON output
```

---

## Wiki — Maintenance

| Command | Description |
|---------|-------------|
| `aura wiki rm <slug>` | Delete a page |
| `aura wiki watch <dir>` | Auto-ingest on file changes |

---

## Setup

| Command | Description |
|---------|-------------|
| `aura setup claude` | Generate MCP config for Claude Code |
| `aura setup cursor` | Generate MCP config for Cursor |
| `aura setup kiro` | Generate MCP config for Kiro |
| `aura version [--json]` | Version, build date, commit |
| `aura completion <shell>` | Shell completions (bash/zsh/fish) |

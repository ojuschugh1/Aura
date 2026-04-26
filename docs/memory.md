---
layout: default
title: Memory
description: Cross-tool persistent memory for AI sessions.
---

# Memory

Aura's memory store is a persistent key-value store shared across all your AI tools. Store decisions, context, and project state once — every MCP-compatible tool reads from the same store.

---

## Basic usage

```bash
# Store context
aura memory add "auth"         "JWT tokens, 24h expiry, refresh via httpOnly cookie"
aura memory add "stack"        "Go backend, React frontend, PostgreSQL"
aura memory add "deploy"       "Docker + Kubernetes on AWS EKS"
aura memory add "api-version"  "v2, REST, OpenAPI 3.0"

# Retrieve
aura memory get "auth"
# key:     auth
# value:   JWT tokens, 24h expiry, refresh via httpOnly cookie
# source:  cli
# updated: 2026-04-26 17:44:00

# List all
aura memory ls
# KEY                            SOURCE             UPDATED              VALUE
# auth                           cli                2026-04-26 17:44     JWT tokens, 24h…
# stack                          cli                2026-04-26 17:44     Go backend, Reac…

# Delete
aura memory rm "api-version"
```

---

## Filtering

```bash
# Show only entries from a specific tool
aura memory ls --agent claude

# Show only auto-captured entries
aura memory ls --auto
```

---

## Auto-capture

Aura automatically extracts decisions from AI session transcripts without any manual effort. When the AI says "we decided to use PostgreSQL" or "going with JWT tokens", Aura captures it:

```bash
aura memory ls --auto
# KEY                            SOURCE             VALUE
# use postgresql                 auto-capture       we decided to use PostgreSQL for…
# jwt tokens                     auto-capture       going with JWT tokens for auth…
```

---

## Export and import

```bash
# Export to JSON
aura memory export
# exported to ~/.aura/memory_export.json

# Export to a specific file
aura memory export --file ./backup/memory.json

# Import
aura memory import --file ./backup/memory.json
# imported 42 entries
```

---

## JSON output

All memory commands support `--json` for scripting:

```bash
aura memory ls --json | jq '.[] | select(.source_tool == "auto-capture")'
aura memory get "auth" --json
```

---

## How memory promotes to wiki

When the daemon is running, important memory entries are automatically promoted to wiki pages every 5 minutes. Entries matching these keywords are promoted:

`architecture`, `decision`, `stack`, `design`, `auth`, `database`, `deploy`, `api`, `config`, `migration`

All auto-captured entries are also promoted. This means your memory and wiki stay in sync automatically.

---

## Multi-agent memory

Multiple AI tools can read and write to the same memory store simultaneously. The last writer wins, with conflict detection:

- Concurrent writes to the same key within 100ms are logged as conflicts
- All agents see writes within 100ms (SQLite WAL mode)
- Activity is logged per agent and session

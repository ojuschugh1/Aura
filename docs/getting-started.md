---
layout: default
title: Getting Started
description: Install Aura and connect it to your AI tools in under 5 minutes.
---

# Getting Started

## Install

### One-line install (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ojuschugh1/Aura/main/install.sh | sh
```

### Using go install

```bash
go install github.com/ojuschugh1/Aura/cmd/aura@latest
```

### Build from source

```bash
git clone https://github.com/ojuschugh1/Aura.git
cd Aura
go build -o aura ./cmd/aura/
```

### Windows

Download the latest `aura-windows-amd64.exe` from [Releases](https://github.com/ojuschugh1/Aura/releases) and add it to your PATH.

---

## Start the daemon

```bash
aura init
```

This starts the background daemon, creates `~/.aura/` with config files, and begins the auto-learning loop. The daemon runs until you stop it with `aura stop`.

Check it's running:

```bash
aura status
# daemon:  running (pid 12345)
# port:    7437
# memory:  0 entries
```

---

## Connect to your AI tools

Aura exposes an MCP server on `localhost:7437`. Add it to your AI tool's config:

```bash
aura setup claude    # Claude Code
aura setup cursor    # Cursor
aura setup kiro      # Kiro
```

Each command prints the JSON snippet to add to your tool's config file. After adding it, restart your AI tool and Aura's memory and wiki tools will be available.

---

## First steps

### Store some context

```bash
aura memory add "stack"    "Go backend, React frontend, PostgreSQL"
aura memory add "auth"     "JWT tokens, 24h expiry, refresh via httpOnly cookie"
aura memory add "deploy"   "Docker + Kubernetes on AWS EKS"
aura memory ls
```

### Verify what your AI did

```bash
aura verify
# Truth score: 83%
# [PASS] created src/auth.ts
# [FAIL] installed jsonwebtoken — not found in lockfile
```

### Build a knowledge wiki

```bash
aura wiki ingest README.md
aura wiki ingest https://your-docs-site.com/architecture
aura wiki query "authentication"
aura wiki viz    # opens wiki-map.html in your browser
```

### Check costs

```bash
aura cost --daily
# input:  45,000 tokens
# cost:   $0.23
# saved:  $0.08 (compression)
```

---

## What happens automatically

Once `aura init` is running, the daemon automatically:

- **Every session end** — ingests the session transcript and creates a summary page in the wiki
- **Every 5 minutes** — promotes important memory entries (architecture, decisions, auth) to wiki pages
- **Every 6 hours** — runs knowledge metabolism: decays stale pages, detects contradictions, flags consolidation candidates

You don't need to do anything. The wiki grows and improves on its own.

---

## Next steps

- [Memory](memory) — learn about cross-tool memory
- [Wiki](wiki) — explore the knowledge wiki features
- [Auto-Learning](autolearn) — understand how Aura learns automatically
- [Commands](commands) — full command reference

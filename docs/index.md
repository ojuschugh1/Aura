---
layout: default
title: Aura — AI Continuity OS
description: Your AI remembers what it did, proves it, and gets smarter every session.
---

# Aura

**Your AI remembers what it did, proves it, and gets smarter every session.**

Aura is a local-first daemon that gives every AI tool you use — Claude Code, Cursor, Kiro, Gemini CLI — persistent memory, claim verification, token compression, dependency scanning, and a self-improving knowledge wiki. One binary. Zero cloud. Works across all tools.

[![CI](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml/badge.svg)](https://github.com/ojuschugh1/Aura/actions/workflows/ci.yml)
[![Release](https://github.com/ojuschugh1/Aura/actions/workflows/release.yml/badge.svg)](https://github.com/ojuschugh1/Aura/releases/latest)

---

## Install in 30 seconds

```bash
curl -fsSL https://raw.githubusercontent.com/ojuschugh1/Aura/main/install.sh | sh
aura init
```

That's it. Aura starts learning from your AI sessions immediately.

---

## Documentation

- [Getting Started](getting-started) — install, first steps, connect to your AI tools
- [Memory](memory) — cross-tool persistent memory
- [Verification](verification) — claim verification and truth scoring
- [Wiki](wiki) — self-improving knowledge base
- [Auto-Learning](autolearn) — how Aura learns automatically
- [Commands](commands) — full command reference
- [Configuration](configuration) — config files and options
- [MCP Integration](mcp) — connecting to AI tools

---

## Why Aura

Every AI tool starts from zero. Your decisions, your context, your reasoning — gone at the session boundary. Aura fixes this by running a local daemon that:

1. **Remembers** — stores decisions and context across all your AI tools
2. **Verifies** — checks whether the AI actually did what it claimed
3. **Learns** — builds a compounding knowledge wiki from every session, automatically

No cloud. No API keys. No IDE plugins required.

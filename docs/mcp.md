---
layout: default
title: MCP Integration
description: Connect Aura to Claude Code, Cursor, Kiro, and other AI tools.
---

# MCP Integration

Aura exposes all its features as MCP (Model Context Protocol) tools on `localhost:7437`. Any MCP-compatible AI tool can use Aura's memory, wiki, verification, and more.

---

## Connect your AI tool

```bash
aura setup claude    # Claude Code
aura setup cursor    # Cursor
aura setup kiro      # Kiro
```

Each command prints the JSON snippet to add to your tool's config file. After adding it and restarting your tool, all Aura tools are available.

---

## Available MCP tools

### Memory tools

| Tool | Description |
|------|-------------|
| `memory_write` | Store a key-value entry |
| `memory_read` | Retrieve an entry by key |
| `memory_list` | List all entries |
| `memory_delete` | Delete an entry |

### Wiki tools

| Tool | Description |
|------|-------------|
| `wiki_ingest` | Ingest a source (title, content, format, origin) |
| `wiki_ingest_url` | Fetch and ingest a URL |
| `wiki_query` | Search pages and get a synthesised answer |
| `wiki_read` | Read a specific page by slug |
| `wiki_write` | Create or update a page |
| `wiki_search` | Search pages by title and content |
| `wiki_index` | Get the full page catalog |
| `wiki_lint` | Health check |
| `wiki_log` | Recent activity log |
| `wiki_graph` | Connectivity stats |
| `wiki_export` | Export as Obsidian markdown |
| `wiki_save_query` | Run a query and save the answer |
| `wiki_schema` | Generate LLM schema |
| `wiki_filter` | Metadata queries |

### Tool pipeline

| Tool | Description |
|------|-------------|
| `wiki_feed_sqz` | Feed SQZ compression stats |
| `wiki_feed_ghostdep` | Feed GhostDep scan results |
| `wiki_feed_claimcheck` | Feed ClaimCheck verification |
| `wiki_feed_etch` | Feed Etch API changes |
| `wiki_feed_json` | Feed any JSON output |

### Other tools

| Tool | Description |
|------|-------------|
| `verify_session` | Verify agent claims |
| `cost_summary` | Token cost summary |
| `compact_context` | Compress context via sqz |
| `scan_deps` | Scan for phantom dependencies |
| `check_action` | Check action against policy |
| `escrow_decide` | Approve or deny an escrow action |
| `route_task` | Route task to appropriate model |
| `trace_summary` | Get session trace summary |

---

## Authentication

Aura uses a shared secret for MCP authentication. The secret is generated on `aura init` and stored in `~/.aura/config.toml`. The `aura setup` commands include the secret in the generated config snippet.

---

## Example: Using wiki tools in Claude Code

After running `aura setup claude` and adding the config, you can ask Claude:

> "Check the wiki for our authentication architecture"

Claude will call `wiki_query` with "authentication" and return the synthesised answer from your wiki pages.

> "Save this decision to the wiki: we're using PostgreSQL with pgvector for semantic search"

Claude will call `wiki_ingest` to store the decision as a wiki source.

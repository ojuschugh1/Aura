---
layout: default
title: Knowledge Wiki
description: A self-improving knowledge base that learns from every AI session.
---

# Knowledge Wiki

The wiki is a persistent, compounding knowledge base that automatically learns from every AI session. Ingest sources once — files, URLs, or tool output — and Aura builds interlinked pages that get richer with every interaction.

**Zero LLM calls.** All extraction is heuristic-based. No API costs.

---

## Ingesting sources

```bash
# From a file
aura wiki ingest design.md
aura wiki ingest architecture.pdf

# From a URL (HTML → markdown conversion)
aura wiki ingest https://example.com/blog/post

# Inline text
aura wiki ingest --title "Auth Decision" "We decided to use JWT with 24h expiry"

# Batch ingest an entire folder
aura wiki ingest --batch ./docs
```

Each ingest:
1. Stores the raw source (immutable)
2. Extracts topics from headers and bold terms
3. Creates/updates wiki pages with cross-references
4. Appends to the activity log
5. Records in the audit chain

---

## Querying

```bash
# Search and synthesise
aura wiki query "authentication"
# found 3 page(s):
# ## Authentication
# JWT tokens with 24-hour expiry...

# Save the answer as a synthesis page
aura wiki query "auth comparison" --save
# saved as wiki page: synthesis-auth-comparison
```

---

## Navigating the knowledge graph

```bash
# Shortest path between two pages
aura wiki trace auth-service database
# auth-service → jwt-tokens → database (2 hops)

# Pages within N hops
aura wiki nearby postgresql --depth 2
# 5 page(s) near postgresql (depth 2):
# redis            entity    1 linked_from
# database-layer   concept   1 links_to

# Full 360° view of a page
aura wiki context auth-service
# confidence: 85% (strong)
# links to (3): jwt-tokens, database, session-mgmt
# linked from (2): api-gateway, user-service
# backed by (2 sources): #1 design.md, #3 auth-rfc.md
```

---

## Visualizing

```bash
aura wiki viz
# knowledge map: ~/.aura/wiki-map.html (42 nodes, 87 edges)
```

Opens an interactive force-directed graph in your browser. Nodes are colored by category, sized by connection count. Hover to see content, search, filter by category.

---

## Health and lifecycle

```bash
# Health check with actionable suggestions
aura wiki lint
# pages:   42   sources: 12   health: 87%
# orphans (2): old-api-notes, unused-concept
# suggestions:
#   [create_page] "redis-caching" is referenced but missing
#   [split_page]  "architecture" has 620 words — consider splitting

# Knowledge lifecycle pass
aura wiki metabolize
# decayed:     3 pages (stale, not accessed)
# consolidate: 2 candidates (overlapping content)
# pressure:    1 alert (3 contradictions on "database-choice")
```

---

## Confidence scoring

Every page has a confidence score (0–100%) based on:
- **Source backing** — more sources = higher confidence
- **Freshness** — recently updated = more trustworthy
- **Cross-reference density** — well-linked = better validated
- **Category** — tool output is deterministic (high confidence)
- **Origin** — auto-extracted content is less certain

```bash
aura wiki context postgresql
# confidence: 85% (strong)
```

Labels: `verified` (90%+), `strong` (75%+), `inferred` (60%+), `weak` (40%+), `uncertain`

---

## Access tiers

Control which agents can see which pages:

```bash
aura wiki access architecture-decision team    # team + private agents only
aura wiki access salary-data private           # owner only
aura wiki access public-docs public            # everyone (default)
```

Tiers: `public` (all agents), `team` (team + private), `private` (owner only)

---

## Audit chain

Every page mutation is recorded in a tamper-evident hash chain:

```bash
aura wiki audit architecture-decision
# [2026-04-26 17:44] create architecture-decision by ingest
# [2026-04-26 18:12] update architecture-decision by ingest

aura wiki verify-chain
# audit chain: 47 entries
# verified:    47
# status:      ✓ intact
```

If any entry is modified or deleted, `verify-chain` detects the break.

---

## Contradiction pressure

When multiple sources contradict a page, pressure accumulates:

```bash
aura wiki pressure database-choice
# pressure on database-choice (3 entries):
#   [contradiction] from migration-plan: Uses PostgreSQL instead
#   [contradiction] from new-arch: Migrated to MongoDB
#   [contradiction] from tech-review: PostgreSQL is the primary DB
```

After 3+ contradictions, `metabolize` raises a revision alert.

---

## Exporting

```bash
# Obsidian-compatible markdown with YAML frontmatter
aura wiki export
# exported 42 pages to ~/.aura/wiki-export
# open in Obsidian to browse with graph view and Dataview queries

# Generate LLM schema
aura wiki schema --format claude > CLAUDE.md
aura wiki schema --format kiro > .kiro/steering/wiki-schema.md
```

---

## Tool pipeline

Feed output from your other tools directly into the wiki:

```bash
aura scan --json | aura wiki feed --tool ghostdep
aura verify --json | aura wiki feed --tool claimcheck
aura wiki feed scan-results.json --tool ghostdep
```

Supported tools: `sqz`, `ghostdep`, `claimcheck`, `etch`, or any JSON via `--tool <name>`

---

## Metadata queries

```bash
aura wiki filter "category=entity"
aura wiki filter "tags contains api"
aura wiki filter "category=entity AND link_count>3"
aura wiki filter "updated>2026-04-01 AND source_count>=2"
```

Fields: `category`, `slug`, `title`, `tags`, `source_count`, `link_count`, `created`, `updated`
Operators: `=`, `!=`, `>`, `<`, `>=`, `<=`, `contains`

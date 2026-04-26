---
layout: default
title: Auto-Learning
description: How Aura automatically learns from every AI session without any manual steps.
---

# Auto-Learning

Aura learns automatically from everything happening on your machine. No IDE plugins, no manual steps, no hooks to configure. Just run `aura init` and it starts learning.

---

## How it works

The auto-learner runs inside the Aura daemon alongside the MCP server. It has four mechanisms:

### 1. Session ingestion (on every session end)

When any AI session ends, the daemon automatically:
- Ingests the session transcript as a wiki source
- Creates a session summary page from memory entries written during the session
- Records the event in the audit chain

```
Session ends → transcript ingested → summary page created → audit recorded
```

### 2. Memory sync (every 5 minutes)

The daemon scans memory entries and promotes important ones to wiki pages. Entries matching these patterns are promoted:

`architecture`, `decision`, `stack`, `design`, `auth`, `database`, `deploy`, `api`, `config`, `migration`

Auto-captured decisions (from `auto-capture`) are always promoted.

```
memory add "architecture" "event sourcing"
→ wiki page "memory-architecture" created automatically
```

### 3. Knowledge metabolism (every 6 hours)

The daemon runs a full lifecycle pass:

- **Decay** — pages not accessed or updated lose vitality over time
- **Boost** — frequently queried pages gain vitality
- **Consolidation** — pages with high content overlap are flagged for merging
- **Pressure** — accumulated contradictions trigger revision alerts
- **Archival** — near-dead pages are flagged for review

### 4. Tool result ingestion (on every MCP tool call)

When tools run via MCP, their results are automatically fed into the wiki:

| Tool | Auto-fed as |
|------|-------------|
| `verify_session` | ClaimCheck report |
| `scan_deps` | GhostDep report |
| `compact_context` | SQZ compression stats |
| `cost_summary` | Cost tracking data |

---

## What gets learned

After a few sessions, your wiki will contain:

- **Architecture decisions** — "we use PostgreSQL", "event sourcing pattern"
- **Session summaries** — what happened in each session, what was decided
- **Verification history** — truth scores over time, which claims failed
- **Dependency findings** — phantom deps that keep appearing
- **Compression stats** — token savings over time

---

## Vitality and decay

Every wiki page has a vitality score (0–100%). Pages that aren't accessed or refreshed gradually lose vitality:

| Condition | Effect |
|-----------|--------|
| Updated within 7 days | No decay |
| Queried recently | Decay slowed by 50% |
| Has source backing | Decay slowed by 30% |
| Tool output page | Decay slowed by 70% |
| Below 10% vitality | Archival suggestion |

Run `aura wiki metabolize` to see the current state, or let the daemon run it automatically every 6 hours.

---

## Contradiction pressure

When multiple sources contradict a page, pressure accumulates. After 3+ contradictions from different sources, a revision alert fires:

```bash
aura wiki pressure database-choice
# 3 unresolved contradictions — revision recommended

aura wiki metabolize
# pressure: 1 alert
#   ⚠ Page "database-choice" has 3 unresolved contradictions
#     from migration-plan, new-arch, tech-review
```

After revising the page, resolve the pressure:

```bash
# (update the page with correct information, then)
# pressure resolves automatically on next lint pass
```

---

## Checking what was learned

```bash
# See all wiki pages
aura wiki ls

# See recent activity
aura wiki log

# See session summaries
aura wiki filter "tags contains session"

# See auto-synced memory entries
aura wiki filter "tags contains auto-synced"

# Full knowledge map
aura wiki viz
```

---

## Disabling auto-learning

Auto-learning is always on when the daemon is running. To pause it, stop the daemon:

```bash
aura stop
```

The wiki persists — nothing is lost. Restart with `aura init` to resume learning.

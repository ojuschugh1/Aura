---
inclusion: auto
---

# Aura Wiki — Auto-Learning Instructions

This project has a built-in knowledge wiki managed by the `aura` CLI.
The wiki accumulates knowledge across sessions — decisions, architecture,
research, tool output — so nothing is lost at the session boundary.

## How to use the wiki in every session

### At the start of a session
1. Run `aura wiki index --dir .aura` to see what knowledge exists
2. Run `aura wiki query "<topic>" --dir .aura` for any topic you need context on
3. The wiki remembers decisions from previous sessions — check before re-deciding

### During work
- When a decision is made, ingest it: `aura wiki ingest --dir .aura --title "Decision: <topic>" "<what was decided and why>"`
- When you find something important, save it: `aura wiki ingest --dir .aura --title "<topic>" "<content>"`
- When you answer a complex question, save the answer: `aura wiki query "<topic>" --save --dir .aura`

### At the end of a session
1. Run `aura wiki lint --dir .aura` to check wiki health
2. Run `aura wiki metabolize --dir .aura` to run the knowledge lifecycle
3. If any contradictions or stale pages are found, address them

### Periodically
- Run `aura wiki viz --dir .aura` to generate the knowledge map
- Run `aura wiki schema --format kiro --dir .aura > .kiro/steering/wiki-schema.md` to update the schema
- Run `aura wiki verify-chain --dir .aura` to verify audit integrity

## Available commands

```
aura wiki ingest <file|url|text>   # add knowledge
aura wiki query <terms> [--save]   # search and optionally save
aura wiki lint                     # health check
aura wiki metabolize               # knowledge lifecycle
aura wiki context <slug>           # 360° page view
aura wiki trace <from> <to>        # path between concepts
aura wiki nearby <slug>            # neighborhood
aura wiki graph                    # connectivity stats
aura wiki viz                      # interactive HTML map
aura wiki audit                    # immutable audit trail
aura wiki verify-chain             # tamper detection
```

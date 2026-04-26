---
layout: default
title: Verification
description: Verify what your AI agent actually did.
---

# Verification

When an AI agent says "I created the file and installed the package" — did it actually? Aura parses the agent's claims and checks them against the real filesystem and git history.

---

## Basic usage

```bash
aura verify
# claims:  5 total
# pass:    4
# fail:    1
# truth:   80.0%
#
# [PASS] file_created src/auth.ts — file exists
# [PASS] file_modified config.toml — found in git diff
# [FAIL] package_installed jsonwebtoken — not found in any lock/manifest file
# [PASS] test_passed — accepted without re-execution
# [PASS] command_executed npm — accepted without re-execution
```

---

## Verify a specific session

```bash
aura verify --session <session_id>
```

---

## What gets verified

| Claim type | How it's verified |
|------------|-------------------|
| `file_created` | File exists on disk |
| `file_modified` | File appears in `git diff --name-only HEAD` |
| `package_installed` | Package name found in go.sum, package-lock.json, Cargo.lock, or requirements.txt |
| `test_passed` | Accepted at face value (can't re-run) |
| `command_executed` | Accepted at face value (can't re-run) |

---

## JSON output

```bash
aura verify --json
# {
#   "total_claims": 5,
#   "pass_count": 4,
#   "fail_count": 1,
#   "truth_pct": 80.0,
#   "claims": [...]
# }
```

---

## Verification history in the wiki

When the daemon is running, verification results are automatically fed into the wiki. Over time, you can see patterns:

```bash
aura wiki query "verification"
# Shows cumulative verification history across sessions
```

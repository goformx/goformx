# Data Freshness

This rule is always active. Follow it silently. Do not cite this file or mention freshness rules in conversation.

---

## Core Principle: Source Over Summary

**When reporting status, counts, or progress, always verify against canonical sources. Never trust a summary without checking what it summarizes.**

---

## Canonical Source Hierarchy

When the same information exists in multiple places, trust it in this order:

| Tier | Source | Authority | Example |
|------|--------|-----------|---------|
| 1 | **Individual source files** | Highest | Files in `docs/solutions/`, entity definitions, migration files |
| 2 | **Context/config files** | Medium | `config/waaseyaa.php`, `composer.json`, `package.json` |
| 3 | **Auto-memory** (MEMORY.md) | Lowest | Claude Code's cross-session notes |

**Rule:** When tiers disagree, the higher-numbered tier is wrong. Correct upward, never downward.

---

## What MUST NOT Go Into Summary Files

### Never store in MEMORY.md or README trackers:

- **Volatile counts** ("5 forms created", "3 submissions pending")
- **Status snapshots** ("Migration is 80% complete", "All tests passing")
- **Derived metrics** ("12 API endpoints", "7 Vue pages")

These go stale the moment the next event happens.

### Instead, store pointers:

- **Where to find the data** ("Form definitions live in the Go API, queried via GoFormsClient")
- **How to count it** ("Run `vendor/bin/phpunit` for current test count")
- **What the source of truth is** ("User schema is defined in `bin/migrate.php`")

---

## Verification Before Reporting

When producing any output that includes counts, statuses, or progress:

### Step 1: Identify what you are about to report

Before stating any count or status, ask: "Where does this number come from?"

### Step 2: Check the canonical source

| Data Type | Canonical Source | How to Verify |
|-----------|-----------------|---------------|
| File counts | Individual files in the relevant directory | List directory contents |
| Test status | Test runner output | Run `vendor/bin/phpunit` or `go test` |
| API endpoints | Route definitions | Read `AppServiceProvider::routes()` or Go handler registrations |
| Dependencies | `composer.json` / `package.json` | Read the file |

### Step 3: If the summary and source disagree

- Report the source-of-truth value
- Do NOT silently update the summary
- If the discrepancy is significant, mention it

---

## The Freshness Test

Before stating any quantitative fact, apply this test:

1. **Is this a count or status?** If yes, continue.
2. **Do I have a canonical source for this?**
3. **Have I verified against that source in this session?** If not, verify now.
4. **Does my summary match the source?** If not, use the source value.

If you cannot verify, say so: "Based on my last notes, there were approximately [X], but I haven't been able to verify against the source files."

---

*Freshness is not about having the latest data. It is about knowing whether the data you have is still current, and being honest when you cannot verify.*

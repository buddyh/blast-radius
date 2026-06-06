---
name: blast-radius
description: Map the full blast radius of a code change before making it — every breaking reference, transitive (ripple) caller, affected test, and historically co-changing file — using the `br` CLI (LSP-backed, cross-language impact analysis). Use this whenever the user is about to rename, change the signature of, or remove a function, method, type, API endpoint, database column, or config/env key, or asks "what breaks if I change X", "what depends on this", "what calls this", "impact analysis", "is this safe to change/refactor", or "check the blast radius" — even if they don't say "blast radius" explicitly. Especially reach for it before refactoring shared code.
---

# Blast Radius

Map everything a code change would touch *before* making it, so you don't ship a broken build or a silent regression. This skill drives **`br`** — a CLI that asks the project's language server (gopls, typescript-language-server, pyright, rust-analyzer, clangd) for accurate references and call hierarchy, then layers on git history. That accuracy matters: it tells `getUser` in *this* package apart from a same-named symbol elsewhere, and it follows the call graph to find *indirect* callers — neither of which a text search can do.

## Prerequisite

`br` must be installed:

```sh
go install github.com/buddyh/blast-radius/cmd/br@latest
```

It also needs the language server for the code being analyzed. `br doctor` lists what's available and prints the install command for anything missing (e.g. `go install golang.org/x/tools/gopls@latest`). If `br` itself isn't installed, tell the user the install line above rather than silently falling back.

## Run it

Identify the target type and run `br analyze`, always with `--json` so you can parse it:

```sh
br analyze "<target>" [path] --json
```

| Target | Example |
| --- | --- |
| symbol (function/method/type) | `br analyze "getUserById" .` |
| precise symbol at a location | `br analyze internal/auth.go:42:6 .` |
| API endpoint | `br analyze "POST /api/users" .` |
| database column | `br analyze users.email . --kind column` |
| config / env key | `br analyze DATABASE_URL .` |

`path` defaults to the current directory — pass the repo root to analyze a different project. Kind is auto-detected (endpoints and ALL_CAPS keys); use `--kind` to force it. Other flags: `--depth N` (transitive caller depth, default 3), `--lang go|typescript|python|rust|cpp`, `--cochange-limit N`.

## Read the JSON

```
target, def (file:line), kind, lang,
breaking[]  {file, line}              — direct references
ripple[]    {file, line, via}         — transitive callers (changed via the named caller)
tests[]     {file, line}              — references inside test files
cochange[]  {file, commits, pct}      — files that historically change together (git)
note, ambiguous[]
```

## Present it

Give the user a decision-ready summary, not a raw dump:

1. **What & where** — the definition (`def`), kind, and the counts.
2. **BREAKING** — direct references that must change *with* the edit. List `file:line`.
3. **RIPPLE** — transitive callers to review for behavior changes (`file:line ← via <caller>`). These compile fine but may behave differently.
4. **TESTS** — re-run these.
5. **CO-CHANGE** — files that historically move together with the target even without a static link (config read by name, sibling modules, cross-layer edits). Flag the high-percentage ones as "probably need a look too."
6. **Risk + change order** — small radius (≤3 breaking): change in place, update consumers in the same commit. Medium: add the new version alongside, migrate, remove. Large: strangler pattern (new symbol + deprecation, phased migration). If the target is shared across what look like service boundaries, note that other repos may be affected — `br` only sees this workspace.

If `ambiguous` is populated, the name matched several symbols — tell the user and offer to re-run with `file:line` to pin the exact one. If `note` is present (e.g. a low-confidence text match for a column), surface it honestly.

## When the tool can't help

If `br` is unavailable, or there's no language server for the language, you can fall back to a manual reference search (ripgrep the target across code, tests, docs, and config) — but **say so**, because that's text-only: it can't follow transitive callers and will produce false positives on same-named symbols. Prefer installing `br` + the language server when the analysis actually matters.

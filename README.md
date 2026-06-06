# blast-radius (`br`)

**Map the blast radius of a code change before you make it.**

Point `br` at a symbol — or an API endpoint, DB column, or config key — and it
reports everything a change would touch:

- **breaking** — direct references (semantic, not grep-by-name)
- **ripple** — *transitive* callers, followed through the call graph
- **tests** — the tests that exercise it
- **co-change** — files that *historically* change together with it (git), the
  coupling static analysis can't see
- a **risk + migration** hint

It's accurate because it drives the language servers you already use (gopls,
typescript-language-server, pyright, rust-analyzer, clangd) for find-references
and call-hierarchy — then layers on text resolution for non-symbol targets and
git history for temporal coupling.

## Install

```sh
go install github.com/buddyh/blast-radius/cmd/br@latest
br doctor          # check which language servers are available
```

## Use

```sh
br analyze getUserById                 # a symbol, anywhere in the repo
br analyze internal/auth.go:42:6       # a symbol at a file:line:col
br analyze "POST /api/users"           # an API endpoint
br analyze users.email --kind column   # a database column
br analyze DATABASE_URL                 # a config / env key (auto-detected)
br analyze getUserById --json          # machine-readable, for tooling/agents
```

Flags: `--depth N` (ripple depth, default 3) · `--kind auto|symbol|endpoint|column|config|text`
· `--lang go|typescript|python|rust|cpp` · `--cochange` / `--cochange-limit` · `--json`.

## How it resolves each target

| Target | How |
| --- | --- |
| symbol, `file:line:col` | language server: references + call hierarchy (transitive) |
| endpoint / column / config key | text scan (these live outside the type system) |
| any target, in a git repo | + git co-change for temporal coupling |

## Language servers

`br` uses whatever you have installed (it also looks in `~/go/bin`, `~/.cargo/bin`, …):

| Language | Server | Install |
| --- | --- | --- |
| Go | gopls | `go install golang.org/x/tools/gopls@latest` |
| TypeScript/JavaScript | typescript-language-server | `npm i -g typescript-language-server typescript` |
| Python | pyright | `npm i -g pyright` |
| Rust | rust-analyzer | `rustup component add rust-analyzer` |
| C/C++ | clangd | `brew install llvm` |

## Use as a Claude skill

`br` ships with an agent skill in [`skill/`](skill/SKILL.md) so Claude (or any
skill-aware agent) reaches for it automatically on "what breaks if I change X",
"what depends on this", or before a refactor. Install it by copying it into your
skills directory:

```sh
cp -r skill ~/.claude/skills/blast-radius     # or ~/.agents/skills/blast-radius
```

## Limits (honest)

- Only as smart as the language server, one language at a time.
- Static call hierarchy can't follow reflection, dynamic dispatch, DI containers,
  or string-built names — `br` flags low-confidence/ambiguous cases.
- Resolution is **per workspace** — it doesn't (yet) cross repo/service
  boundaries. Co-change and text targets help cover what symbols can't.

## License

MIT — see [LICENSE](LICENSE).

# blast-radius (`br`)

**Map the blast radius of a code change before you make it.**

Point `br` at a function, method, type, or symbol and it resolves it through the
language server, then reports every reference and the **transitive callers** a
change would ripple to — classified *breaking / ripple / test* — with a
suggested change order. Real symbol resolution, not grep-by-name: no
false-positives from same-named symbols, and it follows the call graph.

## Why LSP

`br` drives the language servers you already use (gopls, typescript-language-server,
pyright, rust-analyzer, clangd) for accurate, cross-language find-references and
**call hierarchy**. The semantic resolution comes from battle-tested engines —
`br` orchestrates them into an impact report.

## Status

Early. `br doctor` works today; the analysis engine is landing incrementally.

```sh
br doctor                 # which language servers are available
br analyze <symbol> [dir] # map a symbol's blast radius   (coming online)
```

## Install the language servers you need

| Language | Server | Install |
| --- | --- | --- |
| Go | gopls | `go install golang.org/x/tools/gopls@latest` |
| TypeScript/JavaScript | typescript-language-server | `npm i -g typescript-language-server typescript` |
| Python | pyright | `npm i -g pyright` |
| Rust | rust-analyzer | `rustup component add rust-analyzer` |
| C/C++ | clangd | `brew install llvm` |

## License

MIT — see [LICENSE](LICENSE).

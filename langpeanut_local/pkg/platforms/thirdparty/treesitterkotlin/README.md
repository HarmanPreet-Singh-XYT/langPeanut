# Vendored tree-sitter-kotlin

Vendors the generated C parser (`src/parser.c`, `src/scanner.c`,
`src/node-types.json`, `src/tree_sitter/parser.h`) from
[`fwcd/tree-sitter-kotlin`](https://github.com/fwcd/tree-sitter-kotlin) v0.3.2
(MIT licensed — see `LICENSE`), unmodified from upstream.

## Why vendored

The upstream repository does not publish a `bindings/go` package or a
`go.mod` — only the grammar source consumed by other language bindings
(Rust, Node, etc). `binding.go` here is a small original cgo wrapper
(not from upstream) exposing `Language()` the same way the other grammar
packages in `pkg/platforms/thirdparty/` do, so `go build ./...` works
offline from a clean clone per `REPRODUCE.md`.

ABI version 14, compatible with this project's
`github.com/tree-sitter/go-tree-sitter` runtime (supports ABI 13-15).

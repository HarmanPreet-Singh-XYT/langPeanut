# Vendored tree-sitter-swift

This directory vendors the generated C parser for the [`alex-pinkus/tree-sitter-swift`](https://github.com/alex-pinkus/tree-sitter-swift)
grammar (module version `v0.0.0-20260825105852-45e8dcdf09d6`, grammar version `0.7.3`, MIT licensed — see `LICENSE`).

## Why this is vendored instead of imported directly

The published Go module for this grammar ships `grammar.js`, `scanner.c`, and
`node-types.json`, but **not** the generated `src/parser.c` — that file is
normally produced by the grammar's own build step (`tree-sitter generate`)
and appears to have been excluded from this particular tag/commit. Without
it, `go build` fails with `parser.c: file not found`.

`parser.c` was regenerated locally from the upstream `grammar.js` using the
official tree-sitter CLI:

```bash
npx --yes tree-sitter-cli generate
```

This is the same deterministic code-generation step the grammar's own
maintainers run to produce `parser.c` — it is not a workaround or a patch,
just running the grammar's declared build process. The output is fully
reproducible from the upstream `grammar.js` and is checked in here so that
`go build ./...` works offline from a clean clone (per `REPRODUCE.md`)
without requiring Node.js/npx as a build-time dependency.

## Regenerating

If the upstream grammar is bumped, regenerate with:

```bash
go mod download github.com/alex-pinkus/tree-sitter-swift@<new-version>
cp "$(go env GOMODCACHE)/github.com/alex-pinkus/tree-sitter-swift@<new-version>/grammar.js" .
cp "$(go env GOMODCACHE)/github.com/alex-pinkus/tree-sitter-swift@<new-version>/src/scanner.c" src/
npx --yes tree-sitter-cli generate
cp src/parser.c src/node-types.json <this-dir>/src/
```

## ABI compatibility

Generated with tree-sitter CLI producing ABI version 15, matching this
project's `github.com/tree-sitter/go-tree-sitter` runtime (which supports
ABI 13–15). Used the same way as the Dart grammar (`UserNobody14/tree-sitter-dart`)
in `pkg/platforms/flutter_dart.go`.

// Package treesitterkotlin provides a cgo binding to the fwcd/tree-sitter-kotlin
// grammar (v0.3.2, MIT licensed — see LICENSE). Its C sources are vendored
// here because the upstream repository does not publish a bindings/go
// package, only grammar.js/src/*.c for consumption by other language
// bindings (Rust, Node, etc). ABI version 14, compatible with this project's
// github.com/tree-sitter/go-tree-sitter runtime (supports ABI 13-15).
package treesitterkotlin

// #cgo CFLAGS: -std=c11 -fPIC -Isrc
// #include "src/parser.c"
// #include "src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language pointer for Kotlin.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_kotlin())
}

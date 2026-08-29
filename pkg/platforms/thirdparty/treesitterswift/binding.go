// Package treesitterswift provides a cgo binding to a locally-generated
// tree-sitter-swift parser. See README.md in this directory for why the
// generated parser.c is vendored here instead of imported from upstream.
package treesitterswift

// #cgo CFLAGS: -std=c11 -fPIC
// #include "src/parser.c"
// #include "src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language pointer for Swift.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_swift())
}

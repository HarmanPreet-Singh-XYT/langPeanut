module github.com/langPeanut/langpeanut-cloud

go 1.26.2

require (
	github.com/langPeanut/langPeanut v0.0.0-00010101000000-000000000000
	github.com/mattn/go-sqlite3 v1.14.28
)

require (
	github.com/UserNobody14/tree-sitter-dart v0.0.0-20260520003023-a9bdfa3db2fb // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
)

// During co-development, point directly at the local langpeanut_local checkout.
// Drop this replace + pin to a tagged pseudo-version before VPS deploy.
replace github.com/langPeanut/langPeanut => ../langpeanut_local

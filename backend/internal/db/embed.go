// Package db sqlc 生成层。models.go / queries.sql.go 由 `go tool sqlc generate` 产出，
// 手写部分只有本文件——把 schema.sql 嵌进来供 store 运行时迁移。
package db

import _ "embed"

//go:embed schema.sql
var SchemaDDL string

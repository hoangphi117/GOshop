package migrations

import "embed"

// FS holds the event ledger migrations. Services mount them next to their own
// embedded migrations by building a combined fs.FS with io/fs.Sub or by
// pointing golang-migrate at a merged source (see Run for the simple path).
//
//go:embed *.sql
var FS embed.FS

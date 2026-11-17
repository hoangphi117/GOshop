// Command reindex-images rewrites every product image URL in Postgres to its
// edge-relative form.
//
// Why not a migration: the stored value is a deployment concern (host, port,
// bucket base), not schema, and it must be derived from S3_PUBLIC_BASE_URL at
// run time. The search index is refreshed separately by republishing through
// the catalog API, which re-embeds and re-syncs Meilisearch.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"goshop/shared/pkg/utils"
)

func main() {
	ctx := context.Background()
	dsn := os.Getenv("CATALOG_DB_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "CATALOG_DB_DSN is required")
		os.Exit(1)
	}
	base := os.Getenv("S3_PUBLIC_BASE_URL")

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close(ctx) }()

	rows, err := conn.Query(ctx, `SELECT product_id, product_image_url FROM products WHERE product_image_url IS NOT NULL AND product_image_url <> ''`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "select: %v\n", err)
		os.Exit(1)
	}
	type row struct {
		id  int32
		url string
	}
	var toFix []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.url); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			os.Exit(1)
		}
		if clean := utils.PublicMediaPath(base, r.url); clean != r.url {
			toFix = append(toFix, row{id: r.id, url: clean})
		}
	}
	rows.Close()

	for _, r := range toFix {
		if _, err := conn.Exec(ctx, `UPDATE products SET product_image_url = $1 WHERE product_id = $2`, r.url, r.id); err != nil {
			fmt.Fprintf(os.Stderr, "update %d: %v\n", r.id, err)
			os.Exit(1)
		}
	}
	fmt.Printf("rewrote %d product image URLs\n", len(toFix))
}

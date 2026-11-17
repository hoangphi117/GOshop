package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VectorLiteral renders a []float32 as pgvector's text form: '[0.1,0.2,...]'.
//
// pgvector-go can do this through a Scan/Value type, but that needs the vector
// OID registered on every pooled connection (shared/pgx has no AfterConnect
// hook, and this service must not add one for all services). The text form is
// accepted by both `$n::vector` casts and the `<=>` operator, so a plain
// string is enough.
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 10)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// batchChunk bounds one round trip. HNSW index update cost dominates, and 128
// rows keeps a single statement well under the statement timeout.
const batchChunk = 128

func sendBatched(ctx context.Context, pool *pgxpool.Pool, items []ProductVector) error {
	for start := 0; start < len(items); start += batchChunk {
		end := min(start+batchChunk, len(items))
		batch := &pgx.Batch{}
		for _, item := range items[start:end] {
			batch.Queue(upsertSQL, item.ProductUUID, VectorLiteral(item.Vector), item.Model, item.SourceHash)
		}
		br := pool.SendBatch(ctx, batch)
		// Every queued statement must be closed, even after the first error,
		// or the connection is left with unread results and the pool leaks.
		for i := start; i < end; i++ {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return fmt.Errorf("upsert embeddings at %d: %w", i, err)
			}
		}
		if err := br.Close(); err != nil {
			return fmt.Errorf("close batch %d-%d: %w", start, end, err)
		}
	}
	return nil
}

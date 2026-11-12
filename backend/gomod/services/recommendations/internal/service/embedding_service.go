package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"goshop/services/recommendations/internal/embed"
	"goshop/services/recommendations/internal/repository"
	"goshop/shared/pkg/logger"
)

// ModelName is recorded next to every vector. A model change is a full
// re-embedding, and this column is how that is detected.
const ModelName = "all-MiniLM-L6-v2"

// backfillBatch is the number of products per embed call. It is below the
// sidecar's 256 cap to keep a single request's tensor small (16 filled
// sequences at a time stays well inside a 512m container).
const backfillBatch = 64

// EmbeddingService owns product embeddings: the backfill and the lookups
// built on top of them.
type EmbeddingService struct {
	repo     repository.EmbeddingRepository
	embedder embed.Embedder
}

func NewEmbeddingService(repo repository.EmbeddingRepository, embedder embed.Embedder) *EmbeddingService {
	return &EmbeddingService{repo: repo, embedder: embedder}
}

// Backfill embeds every catalog product whose text changed since the last run
// and returns how many vectors were written.
//
// It is idempotent and safe to run at every boot: unchanged products are
// skipped by comparing content hashes, so a restart costs one catalog read.
func (s *EmbeddingService) Backfill(ctx context.Context) (int, error) {
	products, err := s.repo.CatalogProducts(ctx)
	if err != nil {
		return 0, err
	}
	if len(products) == 0 {
		return 0, nil
	}

	existing, err := s.repo.EmbeddedHashes(ctx)
	if err != nil {
		return 0, err
	}

	pending := make([]repository.CatalogProduct, 0, len(products))
	hashes := make(map[string]string, len(products))
	for _, p := range products {
		text := EmbeddingText(p)
		hash := ContentHash(text)
		if existing[p.ProductUUID] == hash {
			continue
		}
		pending = append(pending, p)
		hashes[p.ProductUUID.String()] = hash
	}
	if len(pending) == 0 {
		logger.Log.Info().Int("products", len(products)).Msg("Embedding backfill: everything is current")
		return 0, nil
	}

	written := 0
	for start := 0; start < len(pending); start += backfillBatch {
		end := min(start+backfillBatch, len(pending))
		chunk := pending[start:end]

		texts := make([]string, len(chunk))
		for i, p := range chunk {
			texts[i] = EmbeddingText(p)
		}

		vectors, err := s.embedder.Embed(ctx, texts)
		if err != nil {
			// Partial progress is kept: the next run resumes from the hashes
			// already written, so a flaky sidecar does not restart the backfill.
			return written, fmt.Errorf("embed batch at %d: %w", start, err)
		}

		rows := make([]repository.ProductVector, len(chunk))
		for i, p := range chunk {
			rows[i] = repository.ProductVector{
				ProductUUID: p.ProductUUID,
				Vector:      vectors[i],
				Model:       ModelName,
				SourceHash:  hashes[p.ProductUUID.String()],
			}
		}
		if err := s.repo.Upsert(ctx, rows); err != nil {
			return written, err
		}
		written += len(rows)
	}

	logger.Log.Info().
		Int("embedded", written).
		Int("total", len(products)).
		Msg("Embedding backfill complete")
	return written, nil
}

// EmbedOne refreshes a single product's vector from its current catalog text.
//
// This is the publish path: the catalog calls it after a create/update so a new
// or renamed product is recommendable immediately rather than after the next
// restart. It shares Backfill's rules — same text, same hash — so calling it
// for an unchanged product writes nothing and returns 0.
func (s *EmbeddingService) EmbedOne(ctx context.Context, productUUID string) (int, error) {
	u, err := uuid.Parse(productUUID)
	if err != nil {
		return 0, fmt.Errorf("invalid product uuid: %w", err)
	}

	product, err := s.repo.CatalogProduct(ctx, u)
	if err != nil {
		return 0, err
	}
	if product == nil {
		// Deleted or not sellable: drop the stale vector so the ANN index stops
		// offering a product the catalog no longer serves.
		return 0, s.repo.Delete(ctx, u)
	}

	text := EmbeddingText(*product)
	hash := ContentHash(text)
	existing, err := s.repo.EmbeddedHashes(ctx)
	if err != nil {
		return 0, err
	}
	if existing[u] == hash {
		return 0, nil
	}

	vectors, err := s.embedder.Embed(ctx, []string{text})
	if err != nil {
		return 0, fmt.Errorf("embed product %s: %w", u, err)
	}
	rows := []repository.ProductVector{{
		ProductUUID: u,
		Vector:      vectors[0],
		Model:       ModelName,
		SourceHash:  hash,
	}}
	if err := s.repo.Upsert(ctx, rows); err != nil {
		return 0, err
	}
	return 1, nil
}

// EmbeddingText is the exact string a product is embedded from.//
// Name and category carry most of the signal (a "Gaming Laptop" is close to a
// "Gaming Mouse"); the description adds distinguishing detail. Price, stock and
// rating are deliberately absent: two otherwise identical products would stop
// being similar the moment one went on sale.
func EmbeddingText(p repository.CatalogProduct) string {
	parts := make([]string, 0, 3)
	if n := strings.TrimSpace(p.Name); n != "" {
		parts = append(parts, n)
	}
	if c := strings.TrimSpace(p.Category); c != "" {
		parts = append(parts, c)
	}
	if d := strings.TrimSpace(p.Description); d != "" {
		// 500 runes keeps the token budget dominated by name and category.
		parts = append(parts, truncateRunes(d, 500))
	}
	return strings.Join(parts, ". ")
}

// ContentHash is what makes the backfill incremental: identical text must
// produce an identical hash across restarts and machines.
func ContentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"goshop/services/recommendations/internal/repository"
)

type fakeEmbeddingRepo struct {
	products   []repository.CatalogProduct
	hashes     map[uuid.UUID]string
	upserted   []repository.ProductVector
	deleted    []uuid.UUID
	upsertErr  error
	catalogErr error
}

func (f *fakeEmbeddingRepo) CatalogProducts(context.Context) ([]repository.CatalogProduct, error) {
	return f.products, f.catalogErr
}

// CatalogProduct finds the product in the same slice CatalogProducts serves,
// so the publish-path tests exercise one source of truth.
func (f *fakeEmbeddingRepo) CatalogProduct(_ context.Context, id uuid.UUID) (*repository.CatalogProduct, error) {
	if f.catalogErr != nil {
		return nil, f.catalogErr
	}
	for i, p := range f.products {
		if p.ProductUUID == id {
			return &f.products[i], nil
		}
	}
	return nil, nil
}

func (f *fakeEmbeddingRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeEmbeddingRepo) Upsert(_ context.Context, items []repository.ProductVector) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, items...)
	return nil
}

func (f *fakeEmbeddingRepo) EmbeddedHashes(context.Context) (map[uuid.UUID]string, error) {
	if f.hashes == nil {
		f.hashes = map[uuid.UUID]string{}
	}
	return f.hashes, nil
}

func (f *fakeEmbeddingRepo) Similar(context.Context, uuid.UUID, int) ([]repository.SimilarRow, error) {
	return nil, nil
}

func (f *fakeEmbeddingRepo) NearestTo(context.Context, []float32, []uuid.UUID, int) ([]repository.SimilarRow, error) {
	return nil, nil
}

func (f *fakeEmbeddingRepo) Candidates(context.Context, []float32, []uuid.UUID, int) ([]repository.SimilarRow, error) {
	return nil, nil
}

func (f *fakeEmbeddingRepo) NearestActive(context.Context, []float32, repository.FacetFilter, int) ([]repository.SimilarRow, error) {
	return nil, nil
}

// ActiveCount mirrors ActiveAmong so a test can control the total through
// `products` without a second knob to keep consistent.
func (f *fakeEmbeddingRepo) ActiveCount(ctx context.Context, ids []uuid.UUID, filter repository.FacetFilter) (int, error) {
	if len(ids) == 0 {
		return len(f.products), nil
	}
	active, _ := f.ActiveAmong(ctx, ids, filter)
	return len(active), nil
}

// ActiveAmong treats every product in the catalog slice as sellable, so a test
// that cares about filtering can control it through `products` alone.
func (f *fakeEmbeddingRepo) ActiveAmong(_ context.Context, ids []uuid.UUID, _ repository.FacetFilter) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		for _, p := range f.products {
			if p.ProductUUID == id {
				out[id] = struct{}{}
				break
			}
		}
	}
	return out, nil
}

type fakeEmbedder struct {
	calls   int
	texts   [][]string
	failAt  int
	failErr error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	if f.failAt > 0 && f.calls == f.failAt {
		return nil, f.failErr
	}
	f.texts = append(f.texts, texts)
	out := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 384)
		v[0] = float32(i + 1)
		out[i] = v
	}
	return out, nil
}

func product(name string) repository.CatalogProduct {
	return repository.CatalogProduct{
		ProductUUID: uuid.New(),
		Name:        name,
		Description: "desc",
		Category:    "Laptops",
	}
}

// The publish path is the reason a product created after boot is recommendable
// at all: without it the vector only appears on the next restart.
func TestEmbedOneWritesAVectorForANewProduct(t *testing.T) {
	p := product("Acer Swift")
	repo := &fakeEmbeddingRepo{products: []repository.CatalogProduct{p}}
	emb := &fakeEmbedder{}

	n, err := NewEmbeddingService(repo, emb).EmbedOne(context.Background(), p.ProductUUID.String())
	if err != nil {
		t.Fatalf("EmbedOne: %v", err)
	}
	if n != 1 || len(repo.upserted) != 1 {
		t.Fatalf("expected one vector written, got n=%d upserted=%d", n, len(repo.upserted))
	}
	// Same text rule as the backfill, or the two would fight over the row.
	if want := ContentHash(EmbeddingText(p)); repo.upserted[0].SourceHash != want {
		t.Fatalf("source hash mismatch: got %s want %s", repo.upserted[0].SourceHash, want)
	}
}

// A rename must refresh the vector, otherwise "similar products" keeps matching
// the old name forever.
func TestEmbedOneRefreshesAChangedProduct(t *testing.T) {
	p := product("Acer Swift")
	repo := &fakeEmbeddingRepo{
		products: []repository.CatalogProduct{p},
		hashes:   map[uuid.UUID]string{p.ProductUUID: "stale-hash"},
	}
	emb := &fakeEmbedder{}

	n, err := NewEmbeddingService(repo, emb).EmbedOne(context.Background(), p.ProductUUID.String())
	if err != nil || n != 1 {
		t.Fatalf("expected a rewrite, got n=%d err=%v", n, err)
	}
}

// Calling the publish hook twice must not cost an embedding call the second
// time: the catalog retries, and each retry is a full model forward pass.
func TestEmbedOneIsIdempotent(t *testing.T) {
	p := product("Acer Swift")
	repo := &fakeEmbeddingRepo{
		products: []repository.CatalogProduct{p},
		hashes:   map[uuid.UUID]string{p.ProductUUID: ContentHash(EmbeddingText(p))},
	}
	emb := &fakeEmbedder{}

	n, err := NewEmbeddingService(repo, emb).EmbedOne(context.Background(), p.ProductUUID.String())
	if err != nil || n != 0 {
		t.Fatalf("expected no write, got n=%d err=%v", n, err)
	}
	if emb.calls != 0 {
		t.Fatalf("unchanged product must not hit the embedder, got %d calls", emb.calls)
	}
}

// A deleted product must lose its vector, or the ANN index keeps offering
// something the catalog can no longer describe.
func TestEmbedOneDropsVectorForMissingProduct(t *testing.T) {
	repo := &fakeEmbeddingRepo{}
	emb := &fakeEmbedder{}
	id := uuid.New()

	n, err := NewEmbeddingService(repo, emb).EmbedOne(context.Background(), id.String())
	if err != nil || n != 0 {
		t.Fatalf("expected a no-op delete, got n=%d err=%v", n, err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("expected the vector to be deleted, got %v", repo.deleted)
	}
}

func TestEmbedOneRejectsAMalformedUUID(t *testing.T) {
	svc := NewEmbeddingService(&fakeEmbeddingRepo{}, &fakeEmbedder{})
	if _, err := svc.EmbedOne(context.Background(), "not-a-uuid"); err == nil {
		t.Fatal("a malformed uuid must be a 400, not a silent success")
	}
}

// The whole point of the hash check: a restart must not re-embed 5000 rows.
func TestBackfillSkipsUnchangedProducts(t *testing.T) {
	p := product("Acer Swift")
	repo := &fakeEmbeddingRepo{
		products: []repository.CatalogProduct{p},
		hashes:   map[uuid.UUID]string{p.ProductUUID: ContentHash(EmbeddingText(p))},
	}
	emb := &fakeEmbedder{}

	n, err := NewEmbeddingService(repo, emb).Backfill(context.Background())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 0 || emb.calls != 0 || len(repo.upserted) != 0 {
		t.Fatalf("expected no work, got n=%d calls=%d upserted=%d", n, emb.calls, len(repo.upserted))
	}
}

func TestBackfillEmbedsOnlyChangedProducts(t *testing.T) {
	unchanged := product("Acer Swift")
	changed := product("Dell XPS")
	repo := &fakeEmbeddingRepo{
		products: []repository.CatalogProduct{unchanged, changed},
		hashes: map[uuid.UUID]string{
			unchanged.ProductUUID: ContentHash(EmbeddingText(unchanged)),
			changed.ProductUUID:   "stale-hash",
		},
	}
	emb := &fakeEmbedder{}

	n, err := NewEmbeddingService(repo, emb).Backfill(context.Background())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 1 || len(repo.upserted) != 1 {
		t.Fatalf("expected exactly one product embedded, got n=%d upserted=%d", n, len(repo.upserted))
	}
	if repo.upserted[0].ProductUUID != changed.ProductUUID {
		t.Fatal("embedded the wrong product")
	}
	if repo.upserted[0].SourceHash != ContentHash(EmbeddingText(changed)) {
		t.Fatal("stored hash does not describe the stored text")
	}
}

func TestBackfillStopsAndReportsOnEmbedFailure(t *testing.T) {
	repo := &fakeEmbeddingRepo{
		products: []repository.CatalogProduct{product("a"), product("b")},
		hashes:   map[uuid.UUID]string{},
	}
	emb := &fakeEmbedder{failAt: 1, failErr: errors.New("sidecar down")}

	n, err := NewEmbeddingService(repo, emb).Backfill(context.Background())
	if err == nil {
		t.Fatal("expected the embed failure to surface")
	}
	if n != 0 || len(repo.upserted) != 0 {
		t.Fatalf("nothing should have been written, got n=%d", n)
	}
}

func TestEmbeddingTextOmitsVolatileFields(t *testing.T) {
	base := product("Pixel 9")
	text := EmbeddingText(base)
	if text != "Pixel 9. Laptops. desc" {
		t.Fatalf("unexpected embedding text: %q", text)
	}

	// A price or stock change cannot alter the text, which is what keeps the
	// hash stable and the backfill quiet.
	if EmbeddingText(base) != text {
		t.Fatal("embedding text is not deterministic")
	}
}

func TestContentHashIsStable(t *testing.T) {
	if ContentHash("abc") != ContentHash("abc") {
		t.Fatal("hash is not deterministic")
	}
	if ContentHash("abc") == ContentHash("abd") {
		t.Fatal("hash does not distinguish different text")
	}
}

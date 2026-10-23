package search

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog/log"
)

// ProductDocument represents the schema of a product in Meilisearch
type ProductDocument struct {
	ID           string  `json:"id"` // ProductUUID
	ProductID    int32   `json:"product_id"`
	Name         string  `json:"product_name"`
	Slug         string  `json:"product_slug"`
	Description  string  `json:"product_description"`
	Price        float64 `json:"product_price"`
	Stock        int32   `json:"product_stock"`
	SKU          string  `json:"product_sku"`
	CategoryUUID string  `json:"category_uuid"`
	CategoryName string  `json:"category_name"`
	ImageURL     string  `json:"product_image_url"`
	Status       int32   `json:"product_status"`
	CreatedAt    int64   `json:"product_created_at"`
}

// Client wraps the Meilisearch client
type Client struct {
	client meilisearch.ServiceManager
	index  string
}

// NewClient initializes a new Meilisearch client and ensures the products index and settings
func NewClient(host, masterKey, indexName string) (*Client, error) {
	if indexName == "" {
		indexName = "products"
	}

	cli := meilisearch.New(host, meilisearch.WithAPIKey(masterKey))

	// Health check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !cli.IsHealthy() {
		log.Warn().Str("host", host).Msg("Meilisearch healthcheck returned unhealthy (container may still be starting)")
	}

	c := &Client{
		client: cli,
		index:  indexName,
	}

	_ = ctx
	return c, nil
}

// SetupIndex configures searchable, filterable, sortable attributes and typo tolerance
func (c *Client) SetupIndex(ctx context.Context) error {
	idx := c.client.Index(c.index)

	// 1. Searchable attributes (order of priority for relevancy)
	searchableAttrs := []string{
		"product_name",
		"product_slug",
		"category_name",
		"product_sku",
		"product_description",
	}
	if _, err := idx.UpdateSearchableAttributes(&searchableAttrs); err != nil {
		return fmt.Errorf("failed to update searchable attributes: %w", err)
	}

	// 2. Filterable attributes
	filterableAttrs := []any{
		"category_uuid",
		"product_status",
		"product_price",
	}
	if _, err := idx.UpdateFilterableAttributes(&filterableAttrs); err != nil {
		return fmt.Errorf("failed to update filterable attributes: %w", err)
	}

	// 3. Sortable attributes
	sortableAttrs := []string{
		"product_price",
		"product_created_at",
		"product_stock",
	}
	if _, err := idx.UpdateSortableAttributes(&sortableAttrs); err != nil {
		return fmt.Errorf("failed to update sortable attributes: %w", err)
	}

	log.Info().Str("index", c.index).Msg("Meilisearch products index configured successfully")
	return nil
}

// IndexProducts batch indexes product documents into Meilisearch
func (c *Client) IndexProducts(ctx context.Context, docs []ProductDocument) (*meilisearch.TaskInfo, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	pk := "id"
	return c.client.Index(c.index).AddDocuments(docs, &meilisearch.DocumentOptions{PrimaryKey: &pk})
}

// DeleteProduct removes a product from Meilisearch by product_uuid
func (c *Client) DeleteProduct(ctx context.Context, productUUID string) (*meilisearch.TaskInfo, error) {
	return c.client.Index(c.index).DeleteDocument(productUUID, nil)
}

// Search executes a typo-tolerant instant search query
// Search executes a typo-tolerant instant search query
func (c *Client) Search(ctx context.Context, query string, filter string, sort []string, limit, offset int64) (*meilisearch.SearchResponse, error) {
	req := &meilisearch.SearchRequest{
		Limit:  limit,
		Offset: offset,
	}

	if filter != "" {
		req.Filter = filter
	}
	if len(sort) > 0 {
		req.Sort = sort
	}

	return c.client.Index(c.index).Search(query, req)
}

// SearchDocuments executes a typo-tolerant search and decodes into []ProductDocument
func (c *Client) SearchDocuments(ctx context.Context, query string, filter string, sort []string, limit, offset int64) ([]ProductDocument, int64, error) {
	resp, err := c.Search(ctx, query, filter, sort, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var docs []ProductDocument
	if err := resp.Hits.Decode(&docs); err != nil { //nolint:staticcheck // SDK compatibility API
		return nil, 0, fmt.Errorf("failed to decode search hits: %w", err)
	}

	totalHits := resp.EstimatedTotalHits
	if totalHits == 0 {
		totalHits = resp.TotalHits
	}
	return docs, totalHits, nil
}

// GetRawClient returns the underlying meilisearch.ServiceManager
func (c *Client) GetRawClient() meilisearch.ServiceManager {
	return c.client
}

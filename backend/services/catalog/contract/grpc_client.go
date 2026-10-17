package contract

import (
	"context"
	"fmt"

	catalogv1 "goshop/proto/catalog/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ProductItemInfo struct {
	ProductID           int32
	ProductUUID         string
	ProductName         string
	ProductSlug         string
	ProductDescription  string
	ProductPrice        float64
	ProductStock        int32
	ProductSKU          string
	ProductCategoryID   int32
	ProductCategoryUUID string
	ProductCategoryName string
	ProductImageURL     string
	ProductStatus       int32
	ProductAvgRating    float64
	ProductReviewCount  int32
	ProductCreatedAt    string
}

type CategoryItemInfo struct {
	CategoryID          int32
	CategoryUUID        string
	CategoryName        string
	CategorySlug        string
	CategoryDescription string
	CategoryStatus      int32
	CategoryCreatedAt   string
}

type CatalogClient interface {
	GetProductByUUID(ctx context.Context, u string) (*ProductItemInfo, error)
	GetProductByID(ctx context.Context, id int32) (*ProductItemInfo, error)
	GetProductBySlug(ctx context.Context, slug string) (*ProductItemInfo, error)
	GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]ProductItemInfo, error)
	ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error)
	GetCategoryByUUID(ctx context.Context, u string) (*CategoryItemInfo, error)
	ListCategories(ctx context.Context, activeOnly bool) ([]CategoryItemInfo, error)
	Close() error
}

type GRPCCatalogClient struct {
	client catalogv1.CatalogServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCCatalogClient(target string) (*GRPCCatalogClient, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for catalog at %s: %w", target, err)
	}

	return NewGRPCCatalogClientWithConn(conn), nil
}

func NewGRPCCatalogClientWithConn(conn *grpc.ClientConn) *GRPCCatalogClient {
	return &GRPCCatalogClient{
		client: catalogv1.NewCatalogServiceClient(conn),
		conn:   conn,
	}
}

func (c *GRPCCatalogClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func mapProtoToProductItemInfo(p *catalogv1.ProductInfo) *ProductItemInfo {
	if p == nil {
		return nil
	}
	return &ProductItemInfo{
		ProductID:           p.ProductId,
		ProductUUID:         p.ProductUuid,
		ProductName:         p.ProductName,
		ProductSlug:         p.ProductSlug,
		ProductDescription:  p.ProductDescription,
		ProductPrice:        p.ProductPrice,
		ProductStock:        p.ProductStock,
		ProductSKU:          p.ProductSku,
		ProductCategoryID:   p.ProductCategoryId,
		ProductCategoryUUID: p.ProductCategoryUuid,
		ProductCategoryName: p.ProductCategoryName,
		ProductImageURL:     p.ProductImageUrl,
		ProductStatus:       p.ProductStatus,
		ProductAvgRating:    p.ProductAvgRating,
		ProductReviewCount:  p.ProductReviewCount,
		ProductCreatedAt:    p.ProductCreatedAt,
	}
}

func mapProtoToCategoryItemInfo(cat *catalogv1.CategoryInfo) *CategoryItemInfo {
	if cat == nil {
		return nil
	}
	return &CategoryItemInfo{
		CategoryID:          cat.CategoryId,
		CategoryUUID:        cat.CategoryUuid,
		CategoryName:        cat.CategoryName,
		CategorySlug:        cat.CategorySlug,
		CategoryDescription: cat.CategoryDescription,
		CategoryStatus:      cat.CategoryStatus,
		CategoryCreatedAt:   cat.CategoryCreatedAt,
	}
}

func (c *GRPCCatalogClient) GetProductByUUID(ctx context.Context, u string) (*ProductItemInfo, error) {
	resp, err := c.client.GetProduct(ctx, &catalogv1.GetProductRequest{
		ProductUuid: u,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToProductItemInfo(resp.Product), nil
}

func (c *GRPCCatalogClient) GetProductByID(ctx context.Context, id int32) (*ProductItemInfo, error) {
	resp, err := c.client.GetProduct(ctx, &catalogv1.GetProductRequest{
		ProductId: id,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToProductItemInfo(resp.Product), nil
}

func (c *GRPCCatalogClient) GetProductBySlug(ctx context.Context, slug string) (*ProductItemInfo, error) {
	resp, err := c.client.GetProduct(ctx, &catalogv1.GetProductRequest{
		ProductSlug: slug,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToProductItemInfo(resp.Product), nil
}

func (c *GRPCCatalogClient) GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]ProductItemInfo, error) {
	resp, err := c.client.GetProductsBatch(ctx, &catalogv1.GetProductsBatchRequest{
		ProductUuids: uuids,
		ProductIds:   ids,
	})
	if err != nil {
		return nil, err
	}

	result := make([]ProductItemInfo, len(resp.Products))
	for i, p := range resp.Products {
		result[i] = *mapProtoToProductItemInfo(p)
	}
	return result, nil
}

func (c *GRPCCatalogClient) ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error) {
	resp, err := c.client.ValidateProducts(ctx, &catalogv1.ValidateProductsRequest{
		ProductUuids: uuids,
	})
	if err != nil {
		return false, nil, err
	}
	return resp.Valid, resp.MissingUuids, nil
}

func (c *GRPCCatalogClient) GetCategoryByUUID(ctx context.Context, u string) (*CategoryItemInfo, error) {
	resp, err := c.client.GetCategory(ctx, &catalogv1.GetCategoryRequest{
		CategoryUuid: u,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToCategoryItemInfo(resp.Category), nil
}

func (c *GRPCCatalogClient) ListCategories(ctx context.Context, activeOnly bool) ([]CategoryItemInfo, error) {
	resp, err := c.client.ListCategories(ctx, &catalogv1.ListCategoriesRequest{
		ActiveOnly: activeOnly,
	})
	if err != nil {
		return nil, err
	}

	result := make([]CategoryItemInfo, len(resp.Categories))
	for i, cat := range resp.Categories {
		result[i] = *mapProtoToCategoryItemInfo(cat)
	}
	return result, nil
}

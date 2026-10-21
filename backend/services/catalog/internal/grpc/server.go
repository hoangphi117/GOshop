package grpc

import (
	"context"

	catalogv1 "goshop/proto/catalog/v1"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/service"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CatalogGRPCServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	svc service.CatalogService
}

func NewCatalogGRPCServer(svc service.CatalogService) *CatalogGRPCServer {
	return &CatalogGRPCServer{svc: svc}
}

func mapProductDTOToProto(d *dto.ProductDTO) *catalogv1.ProductInfo {
	if d == nil {
		return nil
	}
	desc := ""
	if d.Description != nil {
		desc = *d.Description
	}
	sku := ""
	if d.SKU != nil {
		sku = *d.SKU
	}
	img := ""
	if d.ImageURL != nil {
		img = *d.ImageURL
	}
	statusInt := int32(1)
	switch d.Status {
	case "Inactive":
		statusInt = 2
	case "Out of Stock":
		statusInt = 3
	}

	return &catalogv1.ProductInfo{
		ProductId:           d.ID,
		ProductUuid:         d.UUID,
		ProductName:         d.Name,
		ProductSlug:         d.Slug,
		ProductDescription:  desc,
		ProductPrice:        d.Price,
		ProductStock:        d.Stock,
		ProductSku:          sku,
		ProductCategoryId:   d.CategoryID,
		ProductCategoryUuid: d.CategoryUUID,
		ProductCategoryName: d.CategoryName,
		ProductImageUrl:     img,
		ProductStatus:       statusInt,
		ProductAvgRating:    d.AvgRating,
		ProductReviewCount:  d.ReviewCount,
		ProductCreatedAt:    d.CreatedAt,
	}
}

func mapCategoryDTOToProto(c *dto.CategoryDTO) *catalogv1.CategoryInfo {
	if c == nil {
		return nil
	}
	desc := ""
	if c.Description != nil {
		desc = *c.Description
	}
	statusInt := int32(1)
	if c.Status == "Inactive" {
		statusInt = 2
	}
	return &catalogv1.CategoryInfo{
		CategoryId:          c.CategoryID,
		CategoryUuid:        c.UUID,
		CategoryName:        c.Name,
		CategorySlug:        c.Slug,
		CategoryDescription: desc,
		CategoryStatus:      statusInt,
		CategoryCreatedAt:   c.CreatedAt,
	}
}

func (s *CatalogGRPCServer) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	var prod *dto.ProductDTO
	var err error

	if req.ProductUuid != "" {
		u, parseErr := uuid.Parse(req.ProductUuid)
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid product UUID")
		}
		prod, err = s.svc.GetProductByUUID(ctx, u)
	} else if req.ProductSlug != "" {
		prod, err = s.svc.GetProductBySlug(ctx, req.ProductSlug)
	} else if req.ProductId > 0 {
		prod, err = s.svc.GetProductByID(ctx, req.ProductId)
	} else {
		return nil, status.Error(codes.InvalidArgument, "must provide product_uuid, product_slug, or product_id")
	}

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "product not found: %v", err)
	}

	return &catalogv1.GetProductResponse{
		Product: mapProductDTOToProto(prod),
	}, nil
}

func (s *CatalogGRPCServer) GetProductsBatch(ctx context.Context, req *catalogv1.GetProductsBatchRequest) (*catalogv1.GetProductsBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	prods, err := s.svc.GetProductsBatch(ctx, req.ProductUuids, req.ProductIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to batch get products: %v", err)
	}

	protoProds := make([]*catalogv1.ProductInfo, len(prods))
	for i := range prods {
		protoProds[i] = mapProductDTOToProto(&prods[i])
	}

	return &catalogv1.GetProductsBatchResponse{
		Products: protoProds,
	}, nil
}

func (s *CatalogGRPCServer) ValidateProducts(ctx context.Context, req *catalogv1.ValidateProductsRequest) (*catalogv1.ValidateProductsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	valid, missing, err := s.svc.ValidateProducts(ctx, req.ProductUuids)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to validate products: %v", err)
	}

	return &catalogv1.ValidateProductsResponse{
		Valid:        valid,
		MissingUuids: missing,
	}, nil
}

func (s *CatalogGRPCServer) GetCategory(ctx context.Context, req *catalogv1.GetCategoryRequest) (*catalogv1.GetCategoryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	var cat *dto.CategoryDTO
	var err error

	if req.CategoryUuid != "" {
		u, parseErr := uuid.Parse(req.CategoryUuid)
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid category UUID")
		}
		cat, err = s.svc.GetCategoryByUUID(ctx, u)
	} else if req.CategorySlug != "" {
		cat, err = s.svc.GetCategoryBySlug(ctx, req.CategorySlug)
	} else if req.CategoryId > 0 {
		cat, err = s.svc.GetCategoryByID(ctx, req.CategoryId)
	} else {
		return nil, status.Error(codes.InvalidArgument, "must provide category_uuid, category_slug, or category_id")
	}

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "category not found: %v", err)
	}

	return &catalogv1.GetCategoryResponse{
		Category: mapCategoryDTOToProto(cat),
	}, nil
}

func (s *CatalogGRPCServer) ListCategories(ctx context.Context, req *catalogv1.ListCategoriesRequest) (*catalogv1.ListCategoriesResponse, error) {
	activeOnly := true
	if req != nil {
		activeOnly = req.ActiveOnly
	}

	cats, err := s.svc.GetAllCategories(ctx, &activeOnly)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list categories: %v", err)
	}

	protoCats := make([]*catalogv1.CategoryInfo, len(cats))
	for i := range cats {
		protoCats[i] = mapCategoryDTOToProto(&cats[i])
	}

	return &catalogv1.ListCategoriesResponse{
		Categories: protoCats,
	}, nil
}

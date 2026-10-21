package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/search"
	"goshop/pkg/utils"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/repository"
	inventorycontract "goshop/services/inventory/contract"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"
	"golang.org/x/text/encoding/charmap"
)

type CatalogService interface {
	// Categories
	CreateCategory(ctx context.Context, input dto.CreateCategoryInput) (*dto.CategoryDTO, error)
	GetCategoryByUUID(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error)
	GetCategoryBySlug(ctx context.Context, slug string) (*dto.CategoryDTO, error)
	GetCategoryByID(ctx context.Context, id int32) (*dto.CategoryDTO, error)
	GetAllCategories(ctx context.Context, activeOnly *bool) ([]dto.CategoryDTO, error)
	UpdateCategory(ctx context.Context, input dto.UpdateCategoryInput) (*dto.CategoryDTO, error)
	SoftDeleteCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error)
	RestoreCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error)
	TrashCategory(ctx context.Context, u uuid.UUID) error

	// Products
	CreateProduct(ctx context.Context, input dto.CreateProductInput) (*dto.ProductDTO, error)
	GetProductByUUID(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error)
	GetProductBySlug(ctx context.Context, slug string) (*dto.ProductDTO, error)
	GetProductByID(ctx context.Context, id int32) (*dto.ProductDTO, error)
	GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]dto.ProductDTO, error)
	ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error)
	GetAllProducts(
		ctx context.Context,
		search, categoryUUID string,
		minPrice, maxPrice, minRating float64,
		status int32,
		orderBy, sort string,
		page, limit int32,
		deleted bool,
	) ([]dto.ProductDTO, int32, error)
	SearchProducts(
		ctx context.Context,
		q, categoryUUID string,
		minPrice, maxPrice, minRating float64,
		sort string,
		page, limit int32,
	) ([]dto.ProductDTO, int64, error)
	SuggestProducts(ctx context.Context, q string, limit int32) ([]dto.ProductSuggestionDTO, error)
	UpdateProduct(ctx context.Context, input dto.UpdateProductInput) (*dto.ProductDTO, error)
	SoftDeleteProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error)
	RestoreProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error)
	TrashProduct(ctx context.Context, u uuid.UUID) error
	ImportProductsCSV(ctx context.Context, file io.Reader) (*dto.ProductCSVImportResponse, error)

	// Outbox & Sync
	StartOutboxWorker(ctx context.Context)
	ProcessOutboxBatch(ctx context.Context) (int, error)
}

type catalogService struct {
	repo            repository.CatalogRepository
	pool            *pgxpool.Pool
	cache           cache.RedisCacheService
	search          *search.Client
	inventoryClient inventorycontract.InventoryClient
	sf              singleflight.Group
}

func NewCatalogService(
	repo repository.CatalogRepository,
	pool *pgxpool.Pool,
	cache cache.RedisCacheService,
	searchClient *search.Client,
	inventoryClient inventorycontract.InventoryClient,
) CatalogService {
	return &catalogService{
		repo:            repo,
		pool:            pool,
		cache:           cache,
		search:          searchClient,
		inventoryClient: inventoryClient,
	}
}

// =============================================================================
// CATEGORIES
// =============================================================================

func (s *catalogService) CreateCategory(ctx context.Context, input dto.CreateCategoryInput) (*dto.CategoryDTO, error) {
	slug, err := generateUniqueSlug(ctx, input.Name, func(slugCtx context.Context, sl string) error {
		_, err := s.repo.GetCategoryBySlug(slugCtx, sl)
		return err
	})
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to generate category slug", err)
	}

	status := int32(1)
	if input.Status > 0 {
		status = input.Status
	}

	cat, err := s.repo.CreateCategory(ctx, catalogsqlc.CreateCategoryParams{
		CategoryName:        input.Name,
		CategorySlug:        slug,
		CategoryDescription: input.Description,
		CategoryStatus:      status,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "category already exists")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to create category", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("categories:*")
	}

	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	cat, err := s.repo.GetCategoryByUUID(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) GetCategoryBySlug(ctx context.Context, slug string) (*dto.CategoryDTO, error) {
	cat, err := s.repo.GetCategoryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) GetCategoryByID(ctx context.Context, id int32) (*dto.CategoryDTO, error) {
	cat, err := s.repo.GetCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) GetAllCategories(ctx context.Context, activeOnly *bool) ([]dto.CategoryDTO, error) {
	cats, err := s.repo.ListCategories(ctx, activeOnly)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to list categories", err)
	}
	return dto.MapCategoriesToDTOs(cats), nil
}

func (s *catalogService) UpdateCategory(ctx context.Context, input dto.UpdateCategoryInput) (*dto.CategoryDTO, error) {
	u, err := uuid.Parse(input.UUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID")
	}

	params := catalogsqlc.UpdateCategoryParams{
		CategoryUuid:        u,
		CategoryName:        input.Name,
		CategoryDescription: input.Description,
		CategoryStatus:      input.Status,
	}

	cat, err := s.repo.UpdateCategory(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to update category", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("categories:*")
	}

	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) SoftDeleteCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	cat, err := s.repo.SoftDeleteCategory(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to delete category", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("categories:*")
	}

	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) RestoreCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	cat, err := s.repo.RestoreCategory(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to restore category", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("categories:*")
	}

	res := dto.MapCategoryToDTO(cat)
	return &res, nil
}

func (s *catalogService) TrashCategory(ctx context.Context, u uuid.UUID) error {
	_, err := s.repo.TrashCategory(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash category", err)
	}

	if s.cache != nil {
		_ = s.cache.Clear("categories:*")
	}

	return nil
}

// =============================================================================
// PRODUCTS
// =============================================================================

func (s *catalogService) enrichStockForDTO(ctx context.Context, d *dto.ProductDTO) {
	if s.inventoryClient == nil || d == nil || d.UUID == "" {
		return
	}
	stocks, err := s.inventoryClient.GetStock(ctx, []string{d.UUID}, nil)
	if err != nil {
		logger.Log.Warn().Err(err).Str("uuid", d.UUID).Msg("Failed to fetch stock from inventory service")
		return
	}
	if len(stocks) > 0 {
		d.Stock = stocks[0].Quantity
	}
}

func (s *catalogService) enrichStockForDTOs(ctx context.Context, dtos []dto.ProductDTO) []dto.ProductDTO {
	if s.inventoryClient == nil || len(dtos) == 0 {
		return dtos
	}
	uuids := make([]string, 0, len(dtos))
	for _, d := range dtos {
		if d.UUID != "" {
			uuids = append(uuids, d.UUID)
		}
	}
	if len(uuids) == 0 {
		return dtos
	}
	stocks, err := s.inventoryClient.GetStock(ctx, uuids, nil)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to batch fetch stocks from inventory service")
		return dtos
	}
	stockMap := make(map[string]int32, len(stocks))
	for _, st := range stocks {
		stockMap[st.ProductUUID] = st.Quantity
	}
	for i := range dtos {
		if qty, ok := stockMap[dtos[i].UUID]; ok {
			dtos[i].Stock = qty
		}
	}
	return dtos
}

func (s *catalogService) CreateProduct(ctx context.Context, input dto.CreateProductInput) (*dto.ProductDTO, error) {
	catUUID, err := uuid.Parse(input.CategoryUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID")
	}

	category, err := s.repo.GetCategoryByUUID(ctx, catUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}

	slug, err := generateUniqueSlug(ctx, input.Name, func(slugCtx context.Context, sl string) error {
		_, err := s.repo.GetProductBySlug(slugCtx, sl)
		return err
	})
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
	}

	status := int32(1)
	if input.Status > 0 {
		status = input.Status
	}

	var sku *string
	if input.SKU != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*input.SKU))
		if trimmed != "" {
			sku = &trimmed
		}
	}

	created, err := s.repo.CreateProduct(ctx, catalogsqlc.CreateProductParams{
		ProductName:        input.Name,
		ProductSlug:        slug,
		ProductDescription: input.Description,
		ProductPrice:       utils.FloatToNumeric(input.Price),
		ProductStock:       &input.Stock,
		ProductSku:         sku,
		ProductCategoryID:  category.CategoryID,
		ProductImageUrl:    input.ImageURL,
		ProductStatus:      status,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to create product", err)
	}

	// Enqueue Outbox event for Meilisearch sync & trigger direct search sync
	if prodRow, err := s.repo.GetProductByUUID(ctx, created.ProductUuid); err == nil {
		doc := mapProductRowToSearchDoc(prodRow)
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", created.ProductUuid.String(), "PRODUCT_UPSERT", doc)
	} else {
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", created.ProductUuid.String(), "PRODUCT_UPSERT", created)
	}
	go s.syncProductToSearch(context.Background(), created.ProductUuid)

	// Adjust initial stock in inventory service
	if s.inventoryClient != nil && input.Stock > 0 {
		_, err := s.inventoryClient.AdjustStock(ctx, inventorycontract.AdjustStockInput{
			ProductUUID:    created.ProductUuid.String(),
			ProductID:      created.ProductID,
			QuantityChange: input.Stock,
			ChangeType:     "INITIAL_STOCK",
		})
		if err != nil {
			logger.Log.Warn().Err(err).Str("uuid", created.ProductUuid.String()).Msg("Failed to initialize stock in inventory service")
		}
	}

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	res := dto.MapCreateProductRowToDTO(created)
	s.enrichStockForDTO(ctx, &res)
	return &res, nil
}

func (s *catalogService) GetProductByUUID(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	prod, err := s.repo.GetProductByUUID(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	res := dto.MapProductRowToDTO(prod)
	s.enrichStockForDTO(ctx, &res)
	return &res, nil
}

func (s *catalogService) GetProductBySlug(ctx context.Context, slug string) (*dto.ProductDTO, error) {
	prod, err := s.repo.GetProductBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	res := dto.ProductDTO{
		ID:           prod.ProductID,
		UUID:         prod.ProductUuid.String(),
		Name:         prod.ProductName,
		Slug:         prod.ProductSlug,
		Description:  prod.ProductDescription,
		Price:        utils.NumericToFloat(prod.ProductPrice),
		Stock:        0,
		SKU:          prod.ProductSku,
		CategoryID:   prod.ProductCategoryID,
		CategoryUUID: prod.CategoryUuid.String(),
		CategoryName: prod.CategoryName,
		ImageURL:     prod.ProductImageUrl,
		Status:       "Active",
		AvgRating:    utils.NumericToFloat(prod.ProductAvgRating),
		ReviewCount:  prod.ProductReviewCount,
		CreatedAt:    prod.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt:    prod.ProductUpdatedAt.Format(time.RFC3339),
	}
	if prod.ProductStock != nil {
		res.Stock = *prod.ProductStock
	}
	s.enrichStockForDTO(ctx, &res)
	return &res, nil
}

func (s *catalogService) GetProductByID(ctx context.Context, id int32) (*dto.ProductDTO, error) {
	prod, err := s.repo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	stock := int32(0)
	if prod.ProductStock != nil {
		stock = *prod.ProductStock
	}

	res := dto.ProductDTO{
		ID:           prod.ProductID,
		UUID:         prod.ProductUuid.String(),
		Name:         prod.ProductName,
		Slug:         prod.ProductSlug,
		Description:  prod.ProductDescription,
		Price:        utils.NumericToFloat(prod.ProductPrice),
		Stock:        stock,
		SKU:          prod.ProductSku,
		CategoryID:   prod.ProductCategoryID,
		CategoryUUID: prod.CategoryUuid.String(),
		CategoryName: prod.CategoryName,
		ImageURL:     prod.ProductImageUrl,
		Status:       "Active",
		AvgRating:    utils.NumericToFloat(prod.ProductAvgRating),
		ReviewCount:  prod.ProductReviewCount,
		CreatedAt:    prod.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt:    prod.ProductUpdatedAt.Format(time.RFC3339),
	}
	s.enrichStockForDTO(ctx, &res)
	return &res, nil
}

func (s *catalogService) GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]dto.ProductDTO, error) {
	var results []dto.ProductDTO

	if len(uuids) > 0 {
		parsedUUIDs := make([]uuid.UUID, 0, len(uuids))
		for _, uStr := range uuids {
			if u, err := uuid.Parse(uStr); err == nil {
				parsedUUIDs = append(parsedUUIDs, u)
			}
		}
		if len(parsedUUIDs) > 0 {
			rows, err := s.repo.GetProductsByUUIDs(ctx, parsedUUIDs)
			if err != nil {
				return nil, utils.WrapError(utils.ErrCodeInternal, "failed to batch fetch by UUIDs", err)
			}
			for _, r := range rows {
				stock := int32(0)
				if r.ProductStock != nil {
					stock = *r.ProductStock
				}
				results = append(results, dto.ProductDTO{
					ID:           r.ProductID,
					UUID:         r.ProductUuid.String(),
					Name:         r.ProductName,
					Slug:         r.ProductSlug,
					Description:  r.ProductDescription,
					Price:        utils.NumericToFloat(r.ProductPrice),
					Stock:        stock,
					SKU:          r.ProductSku,
					CategoryID:   r.ProductCategoryID,
					CategoryUUID: r.CategoryUuid.String(),
					CategoryName: r.CategoryName,
					ImageURL:     r.ProductImageUrl,
					Status:       "Active",
					AvgRating:    utils.NumericToFloat(r.ProductAvgRating),
					ReviewCount:  r.ProductReviewCount,
					CreatedAt:    r.ProductCreatedAt.Format(time.RFC3339),
					UpdatedAt:    r.ProductUpdatedAt.Format(time.RFC3339),
				})
			}
		}
	}

	if len(ids) > 0 {
		rows, err := s.repo.GetProductsByIDs(ctx, ids)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to batch fetch by IDs", err)
		}
		for _, r := range rows {
			stock := int32(0)
			if r.ProductStock != nil {
				stock = *r.ProductStock
			}
			results = append(results, dto.ProductDTO{
				ID:           r.ProductID,
				UUID:         r.ProductUuid.String(),
				Name:         r.ProductName,
				Slug:         r.ProductSlug,
				Description:  r.ProductDescription,
				Price:        utils.NumericToFloat(r.ProductPrice),
				Stock:        stock,
				SKU:          r.ProductSku,
				CategoryID:   r.ProductCategoryID,
				CategoryUUID: r.CategoryUuid.String(),
				CategoryName: r.CategoryName,
				ImageURL:     r.ProductImageUrl,
				Status:       "Active",
				AvgRating:    utils.NumericToFloat(r.ProductAvgRating),
				ReviewCount:  r.ProductReviewCount,
				CreatedAt:    r.ProductCreatedAt.Format(time.RFC3339),
				UpdatedAt:    r.ProductUpdatedAt.Format(time.RFC3339),
			})
		}
	}

	results = s.enrichStockForDTOs(ctx, results)
	return results, nil
}

func (s *catalogService) ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error) {
	if len(uuids) == 0 {
		return true, nil, nil
	}
	parsedUUIDs := make([]uuid.UUID, 0, len(uuids))
	for _, uStr := range uuids {
		u, err := uuid.Parse(uStr)
		if err != nil {
			return false, []string{uStr}, nil
		}
		parsedUUIDs = append(parsedUUIDs, u)
	}

	rows, err := s.repo.GetProductsByUUIDs(ctx, parsedUUIDs)
	if err != nil {
		return false, nil, utils.WrapError(utils.ErrCodeInternal, "failed to validate products", err)
	}

	foundMap := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.ProductStatus == 1 { // Active only
			foundMap[r.ProductUuid.String()] = true
		}
	}

	var missing []string
	for _, uStr := range uuids {
		if !foundMap[uStr] {
			missing = append(missing, uStr)
		}
	}

	return len(missing) == 0, missing, nil
}

func (s *catalogService) GetAllProducts(
	ctx context.Context,
	searchStr, categoryUUID string,
	minPrice, maxPrice, minRating float64,
	status int32,
	orderBy, sort string,
	page, limit int32,
	deleted bool,
) ([]dto.ProductDTO, int32, error) {
	if sort == "" {
		sort = "desc"
	}
	if orderBy == "" {
		orderBy = "product_created_at"
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	cacheKey := fmt.Sprintf("catalog:products:%s:%s:%.2f:%.2f:%.2f:%d:%s:%s:%d:%d:%t",
		searchStr, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, page, limit, deleted)

	var cacheData struct {
		Products []dto.ProductDTO `json:"products"`
		Total    int32            `json:"total"`
	}

	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Products != nil {
			cacheData.Products = s.enrichStockForDTOs(ctx, cacheData.Products)
			return cacheData.Products, cacheData.Total, nil
		}
	}

	type fetchResult struct {
		products []dto.ProductDTO
		total    int64
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		rows, err := s.repo.GetAllProducts(ctx, searchStr, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, limit, offset, deleted)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get all products", err)
		}

		var catUUID pgtype.UUID
		if categoryUUID != "" {
			if u, err := uuid.Parse(categoryUUID); err == nil {
				catUUID = pgtype.UUID{Bytes: u, Valid: true}
			}
		}

		var searchParam *string
		if searchStr != "" {
			searchParam = &searchStr
		}

		var minPriceNum, maxPriceNum, minRatingNum pgtype.Numeric
		if minPrice > 0 {
			minPriceNum = utils.FloatToNumeric(minPrice)
		}
		if maxPrice > 0 {
			maxPriceNum = utils.FloatToNumeric(maxPrice)
		}
		if minRating > 0 {
			minRatingNum = utils.FloatToNumeric(minRating)
		}
		var statusParam *int32
		if status > 0 {
			statusParam = &status
		}

		total, err := s.repo.CountProducts(ctx, catalogsqlc.CountProductsParams{
			Deleted:      &deleted,
			Search:       searchParam,
			CategoryUuid: catUUID,
			MinPrice:     minPriceNum,
			MaxPrice:     maxPriceNum,
			MinRating:    minRatingNum,
			Status:       statusParam,
		})
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to count products", err)
		}

		dtos := make([]dto.ProductDTO, len(rows))
		for i, r := range rows {
			dtos[i] = dto.MapProductRowToDTO(r)
		}
		dtos = s.enrichStockForDTOs(ctx, dtos)

		if s.cache != nil {
			cd := struct {
				Products []dto.ProductDTO `json:"products"`
				Total    int32            `json:"total"`
			}{
				Products: dtos,
				Total:    int32(total),
			}
			if err := s.cache.Set(cacheKey, cd, 5*time.Minute); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to cache catalog products")
			}
		}

		return fetchResult{products: dtos, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	fr := res.(fetchResult)
	fr.products = s.enrichStockForDTOs(ctx, fr.products)
	return fr.products, int32(fr.total), nil
}

func (s *catalogService) SearchProducts(
	ctx context.Context,
	q, categoryUUID string,
	minPrice, maxPrice, minRating float64,
	sort string,
	page, limit int32,
) ([]dto.ProductDTO, int64, error) {
	if s.search == nil {
		list, total, err := s.GetAllProducts(ctx, q, categoryUUID, minPrice, maxPrice, minRating, 1, "product_created_at", "desc", page, limit, false)
		if err != nil {
			return nil, 0, err
		}
		return list, int64(total), nil
	}

	var filters []string
	filters = append(filters, "product_status = 1")
	if categoryUUID != "" {
		filters = append(filters, fmt.Sprintf("category_uuid = '%s'", categoryUUID))
	}
	if minPrice > 0 {
		filters = append(filters, fmt.Sprintf("product_price >= %f", minPrice))
	}
	if maxPrice > 0 && maxPrice >= minPrice {
		filters = append(filters, fmt.Sprintf("product_price <= %f", maxPrice))
	}
	if minRating > 0 {
		filters = append(filters, fmt.Sprintf("product_avg_rating >= %f", minRating))
	}

	filterStr := strings.Join(filters, " AND ")

	var sorts []string
	switch sort {
	case "price:asc":
		sorts = append(sorts, "product_price:asc")
	case "price:desc":
		sorts = append(sorts, "product_price:desc")
	case "rating:desc":
		sorts = append(sorts, "product_avg_rating:desc")
	case "rating:asc":
		sorts = append(sorts, "product_avg_rating:asc")
	default:
		sorts = append(sorts, "product_created_at:desc")
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := int64((page - 1) * limit)

	docs, totalHits, err := s.search.SearchDocuments(ctx, q, filterStr, sorts, int64(limit), offset)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Meilisearch query failed, fallback to postgres")
		list, total, err := s.GetAllProducts(ctx, q, categoryUUID, minPrice, maxPrice, minRating, 1, "product_created_at", "desc", page, limit, false)
		if err != nil {
			return nil, 0, err
		}
		return list, int64(total), nil
	}

	products := make([]dto.ProductDTO, 0, len(docs))
	for _, doc := range docs {
		var createdAtStr string
		if doc.CreatedAt > 0 {
			createdAtStr = time.Unix(doc.CreatedAt, 0).Format(time.RFC3339)
		}
		sku := doc.SKU
		img := doc.ImageURL
		products = append(products, dto.ProductDTO{
			UUID:         doc.ID,
			Name:         doc.Name,
			Slug:         doc.Slug,
			Description:  &doc.Description,
			Price:        doc.Price,
			Stock:        doc.Stock,
			SKU:          &sku,
			CategoryUUID: doc.CategoryUUID,
			CategoryName: doc.CategoryName,
			ImageURL:     &img,
			Status:       "Active",
			CreatedAt:    createdAtStr,
		})
	}

	products = s.enrichStockForDTOs(ctx, products)
	return products, totalHits, nil
}

func (s *catalogService) SuggestProducts(ctx context.Context, q string, limit int32) ([]dto.ProductSuggestionDTO, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	if s.search == nil {
		list, _, err := s.GetAllProducts(ctx, q, "", 0, 0, 0, 1, "product_name", "asc", 1, limit, false)
		if err != nil {
			return nil, err
		}
		suggestions := make([]dto.ProductSuggestionDTO, 0, len(list))
		for _, item := range list {
			suggestions = append(suggestions, dto.ProductSuggestionDTO{
				UUID:         item.UUID,
				Name:         item.Name,
				Slug:         item.Slug,
				Price:        item.Price,
				ImageURL:     item.ImageURL,
				CategoryName: item.CategoryName,
			})
		}
		return suggestions, nil
	}

	docs, _, err := s.search.SearchDocuments(ctx, q, "product_status = 1", nil, int64(limit), 0)
	if err != nil {
		return []dto.ProductSuggestionDTO{}, nil
	}

	suggestions := make([]dto.ProductSuggestionDTO, 0, len(docs))
	for _, doc := range docs {
		img := doc.ImageURL
		suggestions = append(suggestions, dto.ProductSuggestionDTO{
			UUID:         doc.ID,
			Name:         doc.Name,
			Slug:         doc.Slug,
			Price:        doc.Price,
			ImageURL:     &img,
			CategoryName: doc.CategoryName,
		})
	}

	return suggestions, nil
}

func (s *catalogService) UpdateProduct(ctx context.Context, input dto.UpdateProductInput) (*dto.ProductDTO, error) {
	u, err := uuid.Parse(input.UUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID")
	}

	var catID *int32
	if input.CategoryUUID != nil {
		catUUID, err := uuid.Parse(*input.CategoryUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID")
		}
		category, err := s.repo.GetCategoryByUUID(ctx, catUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
		}
		catID = &category.CategoryID
	}

	var slug *string
	if input.Name != nil {
		sl, err := generateUniqueSlug(ctx, *input.Name, func(slugCtx context.Context, checkSlug string) error {
			_, err := s.repo.GetProductBySlug(slugCtx, checkSlug)
			return err
		})
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
		}
		slug = &sl
	}

	var sku *string
	if input.SKU != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*input.SKU))
		if trimmed != "" {
			sku = &trimmed
		}
	}

	var pgPrice pgtype.Numeric
	if input.Price != nil {
		pgPrice = utils.FloatToNumeric(*input.Price)
	}

	updated, err := s.repo.UpdateProduct(ctx, catalogsqlc.UpdateProductParams{
		ProductUuid:        u,
		ProductName:        input.Name,
		ProductDescription: input.Description,
		ProductSlug:        slug,
		ProductPrice:       pgPrice,
		ProductStock:       input.Stock,
		ProductSku:         sku,
		ProductCategoryID:  catID,
		ProductImageUrl:    input.ImageURL,
		ProductStatus:      input.Status,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to update product", err)
	}

	// Enqueue Outbox event & trigger direct search sync
	if prodRow, err := s.repo.GetProductByUUID(ctx, updated.ProductUuid); err == nil {
		doc := mapProductRowToSearchDoc(prodRow)
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", updated.ProductUuid.String(), "PRODUCT_UPSERT", doc)
	} else {
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", updated.ProductUuid.String(), "PRODUCT_UPSERT", updated)
	}
	go s.syncProductToSearch(context.Background(), updated.ProductUuid)

	// Adjust Stock delta in Inventory Service
	if s.inventoryClient != nil && input.Stock != nil {
		newStock := *input.Stock
		currentStock := int32(0)
		stocks, err := s.inventoryClient.GetStock(ctx, []string{updated.ProductUuid.String()}, nil)
		if err == nil && len(stocks) > 0 {
			currentStock = stocks[0].Quantity
		} else if updated.ProductStock != nil {
			currentStock = *updated.ProductStock
		}
		delta := newStock - currentStock
		if delta != 0 {
			note := fmt.Sprintf("Admin adjusted stock from %d to %d", currentStock, newStock)
			_, _ = s.inventoryClient.AdjustStock(ctx, inventorycontract.AdjustStockInput{
				ProductUUID:    updated.ProductUuid.String(),
				ProductID:      updated.ProductID,
				QuantityChange: delta,
				ChangeType:     "ADMIN_ADJUSTMENT",
				Note:           &note,
			})
		}
	}

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	res := dto.MapUpdateProductRowToDTO(updated)
	s.enrichStockForDTO(ctx, &res)
	return &res, nil
}

func (s *catalogService) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	prod, err := s.repo.SoftDeleteProduct(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to delete product", err)
	}

	_, _ = s.repo.InsertOutboxEvent(ctx, "product", u.String(), "PRODUCT_DELETE", map[string]string{"id": u.String()})
	go s.deleteProductFromSearch(context.Background(), u)

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	stock := int32(0)
	if prod.ProductStock != nil {
		stock = *prod.ProductStock
	}

	res := dto.ProductDTO{
		ID:        prod.ProductID,
		UUID:      prod.ProductUuid.String(),
		Name:      prod.ProductName,
		Slug:      prod.ProductSlug,
		Price:     utils.NumericToFloat(prod.ProductPrice),
		Stock:     stock,
		Status:    "Inactive",
		CreatedAt: prod.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt: prod.ProductUpdatedAt.Format(time.RFC3339),
	}
	return &res, nil
}

func (s *catalogService) RestoreProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	prod, err := s.repo.RestoreProduct(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to restore product", err)
	}

	if prodRow, err := s.repo.GetProductByUUID(ctx, u); err == nil {
		doc := mapProductRowToSearchDoc(prodRow)
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", u.String(), "PRODUCT_UPSERT", doc)
	} else {
		_, _ = s.repo.InsertOutboxEvent(ctx, "product", u.String(), "PRODUCT_UPSERT", prod)
	}
	go s.syncProductToSearch(context.Background(), u)

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	stock := int32(0)
	if prod.ProductStock != nil {
		stock = *prod.ProductStock
	}

	res := dto.ProductDTO{
		ID:        prod.ProductID,
		UUID:      prod.ProductUuid.String(),
		Name:      prod.ProductName,
		Slug:      prod.ProductSlug,
		Price:     utils.NumericToFloat(prod.ProductPrice),
		Stock:     stock,
		Status:    "Active",
		CreatedAt: prod.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt: prod.ProductUpdatedAt.Format(time.RFC3339),
	}
	return &res, nil
}

func (s *catalogService) TrashProduct(ctx context.Context, u uuid.UUID) error {
	_, err := s.repo.TrashProduct(ctx, u)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash product", err)
	}

	_, _ = s.repo.InsertOutboxEvent(ctx, "product", u.String(), "PRODUCT_DELETE", map[string]string{"id": u.String()})
	go s.deleteProductFromSearch(context.Background(), u)

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	return nil
}

func (s *catalogService) ImportProductsCSV(ctx context.Context, file io.Reader) (*dto.ProductCSVImportResponse, error) {
	rawBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Failed to read CSV file")
	}

	var utf8Reader io.Reader
	if utf8.Valid(rawBytes) {
		utf8Reader = bytes.NewReader(rawBytes)
	} else {
		decoded1258, err1258 := charmap.Windows1258.NewDecoder().Bytes(rawBytes)
		if err1258 == nil && utf8.Valid(decoded1258) {
			utf8Reader = bytes.NewReader(decoded1258)
		} else {
			decoded1252, err1252 := charmap.Windows1252.NewDecoder().Bytes(rawBytes)
			if err1252 == nil && utf8.Valid(decoded1252) {
				utf8Reader = bytes.NewReader(decoded1252)
			} else {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "File encoding is not valid UTF-8")
			}
		}
	}

	reader := csv.NewReader(utf8Reader)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Empty or invalid CSV file")
	}

	expected := []string{"name", "price", "description", "imageUrl", "category_id", "stock", "sku"}
	if len(headers) < len(expected) {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Missing columns in CSV header")
	}

	successCount := 0
	var csvErrors []dto.CSVRowError
	lineNum := 1

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		lineNum++
		if err != nil {
			csvErrors = append(csvErrors, dto.CSVRowError{
				Row:    lineNum,
				Field:  "row",
				Reason: "Malformed CSV line: " + err.Error(),
			})
			continue
		}

		if len(record) < len(expected) {
			csvErrors = append(csvErrors, dto.CSVRowError{
				Row:    lineNum,
				Field:  "columns",
				Reason: "Not enough columns",
			})
			continue
		}

		name := strings.TrimSpace(record[0])
		priceStr := strings.TrimSpace(record[1])
		desc := strings.TrimSpace(record[2])
		img := strings.TrimSpace(record[3])
		catIDStr := strings.TrimSpace(record[4])
		stockStr := strings.TrimSpace(record[5])
		skuStr := strings.TrimSpace(record[6])

		if name == "" {
			csvErrors = append(csvErrors, dto.CSVRowError{Row: lineNum, Field: "name", Reason: "Name cannot be empty"})
			continue
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || price <= 0 {
			csvErrors = append(csvErrors, dto.CSVRowError{Row: lineNum, Field: "price", Reason: "Invalid price"})
			continue
		}
		catID, err := strconv.Atoi(catIDStr)
		if err != nil || catID <= 0 {
			csvErrors = append(csvErrors, dto.CSVRowError{Row: lineNum, Field: "category_id", Reason: "Invalid category ID"})
			continue
		}
		stock, err := strconv.Atoi(stockStr)
		if err != nil || stock < 0 {
			csvErrors = append(csvErrors, dto.CSVRowError{Row: lineNum, Field: "stock", Reason: "Invalid stock"})
			continue
		}

		slug, err := generateUniqueSlug(ctx, name, func(slugCtx context.Context, sl string) error {
			_, err := s.repo.GetProductBySlug(slugCtx, sl)
			return err
		})
		if err != nil {
			slug = strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-" + uuid.New().String()[:8]
		}

		stock32 := int32(stock)
		var descPtr, imgPtr, skuPtr *string
		if desc != "" {
			descPtr = &desc
		}
		if img != "" {
			imgPtr = &img
		}
		if skuStr != "" {
			skuUpper := strings.ToUpper(skuStr)
			skuPtr = &skuUpper
		}

		created, err := s.repo.CreateProduct(ctx, catalogsqlc.CreateProductParams{
			ProductName:        name,
			ProductSlug:        slug,
			ProductDescription: descPtr,
			ProductPrice:       utils.FloatToNumeric(price),
			ProductStock:       &stock32,
			ProductSku:         skuPtr,
			ProductCategoryID:  int32(catID),
			ProductImageUrl:    imgPtr,
			ProductStatus:      1,
		})
		if err != nil {
			csvErrors = append(csvErrors, dto.CSVRowError{Row: lineNum, Field: "database", Reason: err.Error()})
			continue
		}

		if prodRow, err := s.repo.GetProductByUUID(ctx, created.ProductUuid); err == nil {
			doc := mapProductRowToSearchDoc(prodRow)
			_, _ = s.repo.InsertOutboxEvent(ctx, "product", created.ProductUuid.String(), "PRODUCT_UPSERT", doc)
		} else {
			_, _ = s.repo.InsertOutboxEvent(ctx, "product", created.ProductUuid.String(), "PRODUCT_UPSERT", created)
		}
		go s.syncProductToSearch(context.Background(), created.ProductUuid)
		successCount++
	}

	if s.cache != nil {
		_ = s.cache.Clear("products:*")
	}

	return &dto.ProductCSVImportResponse{
		SuccessCount: successCount,
		ErrorCount:   len(csvErrors),
		Errors:       csvErrors,
	}, nil
}

func generateUniqueSlug(ctx context.Context, name string, existsCheck func(ctx context.Context, slug string) error) (string, error) {
	baseSlug := utils.Slugify(name)
	slug := baseSlug
	counter := 1
	for {
		err := existsCheck(ctx, slug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return slug, nil
			}
			return "", err
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}
}

func mapProductRowToSearchDoc(product catalogsqlc.GetProductByUUIDRow) search.ProductDocument {
	img := ""
	if product.ProductImageUrl != nil {
		img = *product.ProductImageUrl
	}
	desc := ""
	if product.ProductDescription != nil {
		desc = *product.ProductDescription
	}
	sku := ""
	if product.ProductSku != nil {
		sku = *product.ProductSku
	}
	stock := int32(0)
	if product.ProductStock != nil {
		stock = *product.ProductStock
	}
	return search.ProductDocument{
		ID:           product.ProductUuid.String(),
		ProductID:    product.ProductID,
		Name:         product.ProductName,
		Slug:         product.ProductSlug,
		Description:  desc,
		Price:        utils.NumericToFloat(product.ProductPrice),
		Stock:        stock,
		SKU:          sku,
		CategoryUUID: product.CategoryUuid.String(),
		CategoryName: product.CategoryName,
		ImageURL:     img,
		Status:       product.ProductStatus,
		CreatedAt:    product.ProductCreatedAt.Unix(),
	}
}

func (s *catalogService) syncProductToSearch(ctx context.Context, u uuid.UUID) {
	if s.search == nil {
		return
	}
	prod, err := s.repo.GetProductByUUID(ctx, u)
	if err != nil {
		return
	}
	doc := mapProductRowToSearchDoc(prod)
	_, _ = s.search.IndexProducts(ctx, []search.ProductDocument{doc})
}

func (s *catalogService) deleteProductFromSearch(ctx context.Context, u uuid.UUID) {
	if s.search == nil {
		return
	}
	_, _ = s.search.DeleteProduct(ctx, u.String())
}

func (s *catalogService) ProcessOutboxBatch(ctx context.Context) (int, error) {
	if s.search == nil {
		return 0, nil
	}

	events, err := s.repo.GetPendingOutboxEvents(ctx, 50)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	upsertDocs := make([]search.ProductDocument, 0, len(events))
	deleteIDs := make([]string, 0)
	processedIDs := make([]int64, 0, len(events))

	for _, ev := range events {
		switch ev.EventType {
		case "PRODUCT_UPSERT":
			var doc search.ProductDocument
			if err := json.Unmarshal(ev.Payload, &doc); err == nil && doc.ID != "" && doc.Name != "" {
				upsertDocs = append(upsertDocs, doc)
			} else {
				if u, parseErr := uuid.Parse(ev.AggregateID); parseErr == nil {
					if prod, getErr := s.repo.GetProductByUUID(ctx, u); getErr == nil {
						upsertDocs = append(upsertDocs, mapProductRowToSearchDoc(prod))
					}
				}
			}
			processedIDs = append(processedIDs, ev.EventID)

		case "PRODUCT_DELETE":
			deleteIDs = append(deleteIDs, ev.AggregateID)
			processedIDs = append(processedIDs, ev.EventID)

		default:
			processedIDs = append(processedIDs, ev.EventID)
		}
	}

	if len(upsertDocs) > 0 {
		if _, err := s.search.IndexProducts(ctx, upsertDocs); err != nil {
			logger.Log.Error().Err(err).Int("count", len(upsertDocs)).Msg("Catalog outbox: failed to index products")
			return 0, err
		}
	}

	for _, id := range deleteIDs {
		if _, err := s.search.DeleteProduct(ctx, id); err != nil {
			logger.Log.Warn().Err(err).Str("id", id).Msg("Catalog outbox: failed to delete product from search")
		}
	}

	for _, id := range processedIDs {
		_ = s.repo.MarkOutboxEventProcessed(ctx, id)
	}

	return len(processedIDs), nil
}

func (s *catalogService) StartOutboxWorker(ctx context.Context) {
	if s.search == nil {
		return
	}
	logger.Log.Info().Msg("Starting Catalog Meilisearch outbox worker...")
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.ProcessOutboxBatch(ctx)
			}
		}
	}()
}

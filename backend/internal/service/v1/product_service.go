package v1service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/search"
	inventorycontract "goshop/services/inventory/contract"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"golang.org/x/text/encoding/charmap"
)

const (
	ProductStatusActive int32 = 1
)

type productService struct {
	repo            repository.ProductRepository
	categoryRepo    repository.CategoryRepository
	outboxRepo      repository.OutboxRepository
	cache           cache.RedisCacheService
	search          *search.Client
	txManager       db.TxManager
	inventoryClient inventorycontract.InventoryClient
	sf              singleflight.Group
}

func NewProductService(
	repo repository.ProductRepository,
	categoryRepo repository.CategoryRepository,
	outboxRepo repository.OutboxRepository,
	redisClient *redis.Client,
	searchClient *search.Client,
	extras ...any,
) ProductService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	var txMgr db.TxManager
	var invClient inventorycontract.InventoryClient
	for _, extra := range extras {
		switch v := extra.(type) {
		case db.TxManager:
			txMgr = v
		case inventorycontract.InventoryClient:
			invClient = v
		}
	}
	return &productService{
		repo:            repo,
		categoryRepo:    categoryRepo,
		outboxRepo:      outboxRepo,
		cache:           cacheService,
		search:          searchClient,
		txManager:       txMgr,
		inventoryClient: invClient,
	}
}

func (ps *productService) enrichStockForDTO(ctx context.Context, dto *v1dto.ProductDTO) {
	if ps.inventoryClient == nil || dto == nil || dto.UUID == "" {
		return
	}
	stocks, err := ps.inventoryClient.GetStock(ctx, []string{dto.UUID}, nil)
	if err != nil {
		logger.Log.Warn().Err(err).Str("uuid", dto.UUID).Msg("Failed to fetch stock from inventory service")
		return
	}
	if len(stocks) > 0 {
		dto.Stock = stocks[0].Quantity
	}
}

func (ps *productService) enrichStockForDTOs(ctx context.Context, dtos []v1dto.ProductDTO) []v1dto.ProductDTO {
	if ps.inventoryClient == nil || len(dtos) == 0 {
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
	stocks, err := ps.inventoryClient.GetStock(ctx, uuids, nil)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to batch fetch stocks from inventory service")
		return dtos
	}
	stockMap := make(map[string]int32, len(stocks))
	for _, s := range stocks {
		stockMap[s.ProductUUID] = s.Quantity
	}
	for i := range dtos {
		if qty, ok := stockMap[dtos[i].UUID]; ok {
			dtos[i].Stock = qty
		}
	}
	return dtos
}

func (ps *productService) withTx(ctx context.Context, fn func(txCtx context.Context, tx pgx.Tx) error) error {
	if ps.txManager != nil {
		return ps.txManager.WithTx(ctx, func(txCtx context.Context) error {
			tx, _ := db.GetTx(txCtx)
			return fn(txCtx, tx)
		})
	}
	tx, _ := db.GetTx(ctx)
	return fn(ctx, tx)
}

func (ps *productService) enqueueProductUpsertOutbox(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID) error {
	if ps.outboxRepo == nil {
		return nil
	}
	prod, err := ps.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		return fmt.Errorf("failed to get product for outbox: %w", err)
	}
	doc := mapProductRowToSearchDoc(prod)
	if _, err := ps.outboxRepo.Insert(ctx, tx, "product", productUUID.String(), "PRODUCT_UPSERT", doc); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}

func (ps *productService) enqueueProductDeleteOutbox(ctx context.Context, tx pgx.Tx, productUUID uuid.UUID) error {
	if ps.outboxRepo == nil {
		return nil
	}
	if _, err := ps.outboxRepo.Insert(ctx, tx, "product", productUUID.String(), "PRODUCT_DELETE", map[string]string{"id": productUUID.String()}); err != nil {
		return fmt.Errorf("failed to insert delete outbox event: %w", err)
	}
	return nil
}

func mapProductRowToSearchDoc(product sqlc.GetProductByUUIDRow) search.ProductDocument {
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

func (ps *productService) CreateProduct(ctx context.Context, input v1dto.CreateProductInput) (*v1dto.ProductDTO, error) {
	categoryUuid, err := uuid.Parse(input.CategoryUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
	}

	category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	input.Params.ProductCategoryID = category.CategoryID

	if input.Params.ProductStatus == 0 {
		input.Params.ProductStatus = ProductStatusActive
	}

	slug, err := generateUniqueSlug(ctx, input.Params.ProductName, func(slugCtx context.Context, slug string) error {
		_, err := ps.repo.GetBySlug(slugCtx, slug)
		return err
	})
	if err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
	}
	input.Params.ProductSlug = slug

	if input.Params.ProductSku != nil {
		sku := strings.ToUpper(strings.TrimSpace(*input.Params.ProductSku))
		if sku == "" {
			input.Params.ProductSku = nil
		} else {
			input.Params.ProductSku = &sku
		}
	}

	var product sqlc.CreateProductRow
	err = ps.withTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		created, err := ps.repo.Create(txCtx, input.Params)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return utils.NewError(utils.ErrCodeConflict, "product already exists")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to create product", err)
		}
		product = created
		return ps.enqueueProductUpsertOutbox(txCtx, tx, product.ProductUuid)
	})
	if err != nil {
		return nil, err
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	if ps.inventoryClient != nil && product.ProductStock != nil && *product.ProductStock > 0 {
		_, err := ps.inventoryClient.AdjustStock(ctx, inventorycontract.AdjustStockInput{
			ProductUUID:    product.ProductUuid.String(),
			ProductID:      product.ProductID,
			QuantityChange: *product.ProductStock,
			ChangeType:     "INITIAL_STOCK",
		})
		if err != nil {
			logger.Log.Warn().Err(err).Str("uuid", product.ProductUuid.String()).Msg("Failed to initialize stock in inventory service")
		}
	}

	dto := v1dto.MapCreateProductRowToDTO(product)
	ps.enrichStockForDTO(ctx, dto)
	return dto, nil
}

func (ps *productService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]v1dto.ProductDTO, int32, error) {
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
		limitInt := utils.GetIntEnv("LIMIT_ITEM_PER_PAGE", 10)
		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	cacheKey := ps.generateCacheKey(search, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, page, limit, deleted)
	var cacheData struct {
		Products []v1dto.ProductDTO `json:"products"`
		Total    int32              `json:"total"`
	}

	if ps.cache != nil {
		if err := ps.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Products != nil {
			cacheData.Products = ps.enrichStockForDTOs(ctx, cacheData.Products)
			return cacheData.Products, cacheData.Total, nil
		}
	}

	type fetchResult struct {
		products []v1dto.ProductDTO
		total    int64
	}

	res, err, _ := ps.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if ps.cache != nil {
			var cd struct {
				Products []v1dto.ProductDTO `json:"products"`
				Total    int32              `json:"total"`
			}
			if err := ps.cache.Get(cacheKey, &cd); err == nil && cd.Products != nil {
				cd.Products = ps.enrichStockForDTOs(ctx, cd.Products)
				return fetchResult{products: cd.Products, total: int64(cd.Total)}, nil
			}
		}

		listProducts, err := ps.repo.GetAll(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, limit, offset, deleted)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get all products", err)
		}

		total, err := ps.repo.CountProducts(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, deleted)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to count products", err)
		}

		dtos := v1dto.MapListProductsToDTO(listProducts)
		dtos = ps.enrichStockForDTOs(ctx, dtos)

		if ps.cache != nil {
			cd := struct {
				Products []v1dto.ProductDTO `json:"products"`
				Total    int32              `json:"total"`
			}{
				Products: dtos,
				Total:    int32(total),
			}
			if err := ps.cache.Set(cacheKey, cd, 5*time.Minute); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to cache products")
			}
		}

		return fetchResult{products: dtos, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	fr := res.(fetchResult)
	fr.products = ps.enrichStockForDTOs(ctx, fr.products)
	return fr.products, int32(fr.total), nil
}

func (ps *productService) GetProductByUUID(ctx context.Context, productUUID uuid.UUID) (*v1dto.ProductDTO, error) {
	product, err := ps.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	dto := v1dto.MapGetProductByUUIDRowToDTO(product)
	ps.enrichStockForDTO(ctx, dto)
	return dto, nil
}

func (ps *productService) GetProductBySlug(ctx context.Context, slug string) (*v1dto.ProductDTO, error) {
	product, err := ps.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	dto := v1dto.MapGetProductBySlugRowToDTO(product)
	ps.enrichStockForDTO(ctx, dto)
	return dto, nil
}

func (ps *productService) UpdateProduct(ctx context.Context, input v1dto.UpdateProductInput) (*v1dto.ProductDTO, error) {
	if input.CategoryUUID != nil {
		categoryUuid, err := uuid.Parse(*input.CategoryUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
		}

		category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil, utils.NewError(utils.ErrCodeNotFound, "category not found")
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
		}
		input.Params.ProductCategoryID = &category.CategoryID
	}

	if input.Params.ProductName != nil {
		slug, err := generateUniqueSlug(ctx, *input.Params.ProductName, func(slugCtx context.Context, slug string) error {
			_, err := ps.repo.GetBySlug(slugCtx, slug)
			return err
		})
		if err != nil {
			var appErr *utils.AppError
			if errors.As(err, &appErr) {
				return nil, err
			}
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
		}

		input.Params.ProductSlug = &slug
	}

	if input.Params.ProductSku != nil {
		sku := strings.ToUpper(strings.TrimSpace(*input.Params.ProductSku))
		if sku == "" {
			input.Params.ProductSku = nil
		} else {
			input.Params.ProductSku = &sku
		}
	}

	var product sqlc.UpdateProductRow
	err := ps.withTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		updated, err := ps.repo.Update(txCtx, input.Params)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return utils.NewError(utils.ErrCodeNotFound, "product not found")
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return utils.NewError(utils.ErrCodeConflict, "product already exists")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to update product", err)
		}
		product = updated
		return ps.enqueueProductUpsertOutbox(txCtx, tx, product.ProductUuid)
	})
	if err != nil {
		return nil, err
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	if ps.inventoryClient != nil && input.Params.ProductStock != nil {
		newStock := *input.Params.ProductStock
		currentStock := int32(0)
		stocks, err := ps.inventoryClient.GetStock(ctx, []string{product.ProductUuid.String()}, nil)
		if err == nil && len(stocks) > 0 {
			currentStock = stocks[0].Quantity
		} else if product.ProductStock != nil {
			currentStock = *product.ProductStock
		}
		delta := newStock - currentStock
		if delta != 0 {
			note := fmt.Sprintf("Admin updated stock from %d to %d", currentStock, newStock)
			_, err := ps.inventoryClient.AdjustStock(ctx, inventorycontract.AdjustStockInput{
				ProductUUID:    product.ProductUuid.String(),
				ProductID:      product.ProductID,
				QuantityChange: delta,
				ChangeType:     "ADMIN_ADJUSTMENT",
				Note:           &note,
			})
			if err != nil {
				logger.Log.Warn().Err(err).Str("uuid", product.ProductUuid.String()).Msg("Failed to adjust stock in inventory service")
			}
		}
	}

	dto := v1dto.MapUpdateProductRowToDTO(product)
	ps.enrichStockForDTO(ctx, dto)
	return dto, nil
}

func (ps *productService) SoftDeleteProduct(ctx context.Context, productUUID uuid.UUID) (*v1dto.ProductDTO, error) {
	var product sqlc.Product
	err := ps.withTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		deleted, err := ps.repo.SoftDelete(txCtx, productUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return utils.NewError(utils.ErrCodeNotFound, "product not found")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to soft delete product", err)
		}
		product = deleted
		return ps.enqueueProductDeleteOutbox(txCtx, tx, productUUID)
	})
	if err != nil {
		return nil, err
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return v1dto.MapProductToDTO(product), nil
}

func (ps *productService) RestoreProduct(ctx context.Context, productUUID uuid.UUID) (*v1dto.ProductDTO, error) {
	var product sqlc.Product
	err := ps.withTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		restored, err := ps.repo.Restore(txCtx, productUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return utils.NewError(utils.ErrCodeNotFound, "product not found")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to restore product", err)
		}
		product = restored
		return ps.enqueueProductUpsertOutbox(txCtx, tx, productUUID)
	})
	if err != nil {
		return nil, err
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return v1dto.MapProductToDTO(product), nil
}

func (ps *productService) TrashProduct(ctx context.Context, productUUID uuid.UUID) error {
	err := ps.withTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		_, err := ps.repo.Trash(txCtx, productUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return utils.NewError(utils.ErrCodeNotFound, "product not found")
			}
			return utils.WrapError(utils.ErrCodeInternal, "failed to trash product", err)
		}
		return ps.enqueueProductDeleteOutbox(txCtx, tx, productUUID)
	})
	if err != nil {
		return err
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return nil
}

func (ps *productService) generateCacheKey(search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) string {
	search = strings.TrimSpace(search)
	if search == "" {
		search = "none"
	}

	categoryUUID = strings.TrimSpace(categoryUUID)
	if categoryUUID == "" {
		categoryUUID = "none"
	}

	minPriceStr := fmt.Sprintf("%.2f", minPrice)
	maxPriceStr := fmt.Sprintf("%.2f", maxPrice)
	minRatingStr := fmt.Sprintf("%.2f", minRating)

	statusStr := fmt.Sprintf("%d", status)

	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		orderBy = "product_created_at"
	}

	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		sort = "desc"
	}
	return fmt.Sprintf("products:%s:%s:%s:%s:%s:%s:%s:%s:%d:%d:%t", search, categoryUUID, minPriceStr, maxPriceStr, minRatingStr, statusStr, orderBy, sort, page, limit, deleted)
}

type ParsedProductRow struct {
	RowNum      int
	Name        string
	Price       float64
	Description *string
	ImageURL    *string
	CategoryID  int32
	Stock       int32
	SKU         string
}

func (ps *productService) ImportProductsCSV(ctx context.Context, file io.Reader) (*v1dto.ProductCSVImportResponse, error) {
	rawBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Failed to read CSV file")
	}

	var utf8Reader io.Reader
	if utf8.Valid(rawBytes) {
		utf8Reader = bytes.NewReader(rawBytes)
	} else {
		// Attempt to auto-decode Windows-1258 (Excel on Windows in Vietnamese locale)
		decoded1258, err1258 := charmap.Windows1258.NewDecoder().Bytes(rawBytes)
		if err1258 == nil && utf8.Valid(decoded1258) {
			utf8Reader = bytes.NewReader(decoded1258)
		} else {
			// Attempt to auto-decode Windows-1252 (Western European ANSI)
			decoded1252, err1252 := charmap.Windows1252.NewDecoder().Bytes(rawBytes)
			if err1252 == nil && utf8.Valid(decoded1252) {
				utf8Reader = bytes.NewReader(decoded1252)
			} else {
				return nil, utils.NewError(utils.ErrCodeBadRequest, "File encoding is not valid UTF-8. Please re-save the CSV file in Excel as 'CSV UTF-8 (Comma delimited)'")
			}
		}
	}

	reader := csv.NewReader(utf8Reader)
	reader.FieldsPerRecord = -1 // Allow variable fields check manually
	reader.LazyQuotes = false   // Strict RFC 4180 compliance
	reader.TrimLeadingSpace = true

	// Validate header
	headers, err := reader.Read()
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Empty or invalid csv file")
	}

	if err := ps.validateCSVHeaders(headers); err != nil {
		return nil, err
	}

	// Parse rows
	parsedRow, syntaxErrors := ps.parseCSVRow(reader)

	// In-file duplicate checks
	duplicateErrors := ps.validateInFileConstraints(parsedRow)
	allErrors := append(syntaxErrors, duplicateErrors...)

	// ALL-OR-NOTHING: If there are any errors, stop processing and return errors
	if len(allErrors) > 0 {
		return &v1dto.ProductCSVImportResponse{
			SuccessCount: 0,
			ErrorCount:   len(allErrors),
			Errors:       allErrors,
		}, nil
	}

	// Validate foreign key constraints (category existence)
	catErrors, err := ps.validateCategoryExistence(ctx, parsedRow)
	if err != nil {
		return nil, err
	}
	if len(catErrors) > 0 {
		return &v1dto.ProductCSVImportResponse{
			SuccessCount: 0,
			ErrorCount:   len(catErrors),
			Errors:       catErrors,
		}, nil
	}

	// Execute atomic batch insert
	insertErr := ps.executeAtomicBatchInsert(ctx, parsedRow)
	if insertErr != nil {
		return &v1dto.ProductCSVImportResponse{
			SuccessCount: 0,
			ErrorCount:   1,
			Errors: []v1dto.CSVRowError{
				{
					Row:    insertErr.Row,
					Field:  insertErr.Field,
					Reason: insertErr.Reason,
				},
			},
		}, nil
	}

	if ps.cache != nil {
		_ = ps.cache.Clear("products:*")
	}

	return &v1dto.ProductCSVImportResponse{
		SuccessCount: len(parsedRow),
		ErrorCount:   0,
		Errors:       []v1dto.CSVRowError{},
	}, nil
}

func (ps *productService) validateCSVHeaders(headers []string) error {
	expected := []string{"name", "price", "description", "imageUrl", "category_id", "stock", "sku"}
	if len(headers) != len(expected) {
		return utils.NewError(utils.ErrCodeBadRequest, "Missing or extra columns in CSV header")
	}

	for i, col := range expected {
		val := strings.TrimSpace(headers[i])
		if i == 0 {
			val = strings.TrimPrefix(val, "\ufeff")
		}
		if !strings.EqualFold(val, col) {
			return utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Expected column '%s' at position %d, but got '%s'", col, i+1, headers[i]))
		}
	}
	return nil
}

func (ps *productService) parseCSVRow(reader *csv.Reader) ([]ParsedProductRow, []v1dto.CSVRowError) {
	var rows []ParsedProductRow
	var rowErrors []v1dto.CSVRowError
	lineNum := 1 // Header is line 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++

		if err != nil {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Reason: fmt.Sprintf("RFC 4180 syntax error: %v", err),
			})
			continue
		}

		if len(record) != 7 {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Reason: "Incorrect number of columns. Expected 7, got " + fmt.Sprintf("%d", len(record)),
			})
			continue
		}

		fieldNames := []string{"name", "price", "description", "imageUrl", "category_id", "stock", "sku"}
		hasEncodingError := false
		for idx, fieldName := range fieldNames {
			if !utf8.ValidString(record[idx]) {
				rowErrors = append(rowErrors, v1dto.CSVRowError{
					Row: lineNum, Field: fieldName, Reason: "Invalid UTF-8 character encoding",
				})
				hasEncodingError = true
			}
		}
		if hasEncodingError {
			continue
		}

		name := strings.TrimSpace(record[0])
		if name == "" {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Field: "name", Reason: "Name is required",
			})
		} else if len(name) < 2 || len(name) > 200 {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Field: "name", Reason: "Name must be between 2 and 200 characters",
			})
		}

		price, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		if err != nil || price <= 0 {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Field: "price", Reason: "Invalid price",
			})
		}

		var descPtr *string
		if desc := strings.TrimSpace(record[2]); desc != "" {
			descPtr = &desc
		}

		var imgPtr *string
		if img := strings.TrimSpace(record[3]); img != "" {
			imgPtr = &img
		}

		categoryID, err := strconv.Atoi(strings.TrimSpace(record[4]))
		if err != nil || categoryID <= 0 {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Field: "category_id", Reason: "Invalid category_id",
			})
		}

		stock := int32(0)
		stockStr := strings.TrimSpace(record[5])
		if stockStr != "" {
			parsedStock, err := strconv.Atoi(stockStr)
			if err != nil || parsedStock < 0 {
				rowErrors = append(rowErrors, v1dto.CSVRowError{
					Row: lineNum, Field: "stock", Reason: "Invalid stock value",
				})
			} else {
				stock = int32(parsedStock)
			}
		}

		sku := strings.TrimSpace(record[6])
		if sku == "" {
			sku = fmt.Sprintf("SKU-%s", strings.ToUpper(uuid.New().String()[:8]))
		} else if len(sku) > 50 {
			rowErrors = append(rowErrors, v1dto.CSVRowError{
				Row: lineNum, Field: "sku", Reason: "SKU must be no more than 50 characters",
			})
		}

		rows = append(rows, ParsedProductRow{
			RowNum:      lineNum,
			Name:        name,
			Price:       price,
			Description: descPtr,
			ImageURL:    imgPtr,
			CategoryID:  int32(categoryID),
			Stock:       stock,
			SKU:         sku,
		})
	}

	return rows, rowErrors
}

// In-File duplicate checks for name-category_id and SKU
func (ps *productService) validateInFileConstraints(rows []ParsedProductRow) []v1dto.CSVRowError {
	var errors []v1dto.CSVRowError
	seenNameCategory := make(map[string]int) // key: name|categoryID, value: rowNum
	seenSKU := make(map[string]int)          // key: sku, value: rowNum

	for _, row := range rows {
		if row.Name == "" || row.CategoryID <= 0 {
			continue // Skip rows that already have errors
		}

		nameCatKey := fmt.Sprintf("%s|%d", strings.ToLower(row.Name), row.CategoryID)
		if prevRow, exists := seenNameCategory[nameCatKey]; exists {
			errors = append(errors, v1dto.CSVRowError{
				Row:    row.RowNum,
				Field:  "name",
				Reason: fmt.Sprintf("Duplicate name and category_id combination found in row %d", prevRow),
			})
		} else {
			seenNameCategory[nameCatKey] = row.RowNum
		}

		if row.SKU != "" {
			if prevRow, exists := seenSKU[row.SKU]; exists {
				errors = append(errors, v1dto.CSVRowError{
					Row:    row.RowNum,
					Field:  "sku",
					Reason: fmt.Sprintf("Duplicate SKU found in row %d", prevRow),
				})
			} else {
				seenSKU[row.SKU] = row.RowNum
			}
		}
	}
	return errors
}

func (ps *productService) validateCategoryExistence(ctx context.Context, rows []ParsedProductRow) ([]v1dto.CSVRowError, error) {
	uniqueCatIDs := make(map[int32]struct{})
	for _, r := range rows {
		if r.CategoryID > 0 {
			uniqueCatIDs[r.CategoryID] = struct{}{}
		}
	}
	categories, err := ps.categoryRepo.GetAll(ctx, "", "", "asc", 500, 0, false)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Không thể tải danh sách danh mục", err)
	}
	validMap := make(map[int32]bool)
	for _, c := range categories {
		validMap[c.CategoryID] = true
	}
	var errors []v1dto.CSVRowError
	for _, row := range rows {
		if !validMap[row.CategoryID] {
			errors = append(errors, v1dto.CSVRowError{
				Row:    row.RowNum,
				Field:  "category_id",
				Reason: fmt.Sprintf("Danh mục có ID = %d không tồn tại trên hệ thống", row.CategoryID),
			})
		}
	}
	return errors, nil
}

type DBInsertError struct {
	Row    int
	Field  string
	Reason string
}

func (ps *productService) executeAtomicBatchInsert(ctx context.Context, rows []ParsedProductRow) *DBInsertError {
	tx, err := db.BeginTx(ctx)
	if err != nil {
		return &DBInsertError{Row: 0, Reason: fmt.Sprintf("Failed to begin transaction: %v", err)}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := sqlc.New(tx)
	for _, row := range rows {
		slug := fmt.Sprintf("%s-%s", utils.Slugify(row.Name), uuid.New().String()[:8])
		skuCopy := row.SKU

		prodCreated, err := qtx.CreateProduct(ctx, sqlc.CreateProductParams{
			ProductName:        row.Name,
			ProductSlug:        slug,
			ProductPrice:       utils.FloatToNumeric(row.Price),
			ProductDescription: row.Description,
			ProductImageUrl:    row.ImageURL,
			ProductCategoryID:  row.CategoryID,
			ProductStock:       &row.Stock,
			ProductSku:         &skuCopy,
			ProductStatus:      ProductStatusActive,
		})

		if err != nil {
			var reason string
			var field string
			var pgErr *pgconn.PgError

			// Catch unique constraint violations
			if errors.As(err, &pgErr) {
				switch pgErr.ConstraintName {
				case "idx_products_name_category_unique":
					field = "name"
					reason = fmt.Sprintf("Duplicate name and category_id combination found: %s", row.Name)
				case "products_product_sku_key":
					field = "sku"
					reason = fmt.Sprintf("Duplicate SKU found: %s", row.SKU)
				default:
					reason = fmt.Sprintf("Database error: %v", err)
				}
			} else {
				reason = err.Error()
			}
			return &DBInsertError{Row: row.RowNum, Field: field, Reason: reason}
		}

		if ps.outboxRepo != nil {
			desc := ""
			if row.Description != nil {
				desc = *row.Description
			}
			img := ""
			if row.ImageURL != nil {
				img = *row.ImageURL
			}
			doc := search.ProductDocument{
				ID:          prodCreated.ProductUuid.String(),
				ProductID:   prodCreated.ProductID,
				Name:        prodCreated.ProductName,
				Slug:        prodCreated.ProductSlug,
				Description: desc,
				Price:       row.Price,
				Stock:       row.Stock,
				SKU:         skuCopy,
				ImageURL:    img,
				Status:      ProductStatusActive,
				CreatedAt:   time.Now().Unix(),
			}
			if _, err := ps.outboxRepo.Insert(ctx, tx, "product", prodCreated.ProductUuid.String(), "PRODUCT_UPSERT", doc); err != nil {
				return &DBInsertError{Row: row.RowNum, Reason: fmt.Sprintf("Failed to enqueue search outbox event: %v", err)}
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return &DBInsertError{Row: 0, Reason: fmt.Sprintf("Failed to commit transaction: %v", err)}
	}

	return nil
}

// SearchProducts executes typo-tolerant search in Meilisearch with fallback/caching
func (ps *productService) SearchProducts(ctx context.Context, q, categoryUUID string, minPrice, maxPrice, minRating float64, sort string, page, limit int32) ([]v1dto.ProductDTO, int64, error) {
	if ps.search == nil {
		// Fallback to PostgreSQL if Meilisearch client is not configured
		list, total, err := ps.GetAllProducts(ctx, q, categoryUUID, minPrice, maxPrice, minRating, ProductStatusActive, "product_created_at", "desc", page, limit, false)
		if err != nil {
			return nil, 0, err
		}
		return list, int64(total), nil
	}

	// 1. Build Meilisearch Filter string
	var filters []string
	filters = append(filters, "product_status = 1") // Only active products
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

	// 2. Build Sort parameters
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
	case "created_at:desc":
		sorts = append(sorts, "product_created_at:desc")
	case "created_at:asc":
		sorts = append(sorts, "product_created_at:asc")
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := int64((page - 1) * limit)

	// 3. Query Meilisearch
	docs, totalHits, err := ps.search.SearchDocuments(ctx, q, filterStr, sorts, int64(limit), offset)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Meilisearch search query failed, falling back to PostgreSQL")
		list, total, err := ps.GetAllProducts(ctx, q, categoryUUID, minPrice, maxPrice, minRating, ProductStatusActive, "product_created_at", "desc", page, limit, false)
		if err != nil {
			return nil, 0, err
		}
		return list, int64(total), nil
	}

	// 4. Map search docs to DTO
	products := make([]v1dto.ProductDTO, 0, len(docs))
	for _, doc := range docs {
		var createdAtStr string
		if doc.CreatedAt > 0 {
			createdAtStr = time.Unix(doc.CreatedAt, 0).Format(time.RFC3339)
		}

		products = append(products, v1dto.ProductDTO{
			UUID:         doc.ID,
			Name:         doc.Name,
			Slug:         doc.Slug,
			Description:  doc.Description,
			Price:        doc.Price,
			Stock:        doc.Stock,
			SKU:          doc.SKU,
			CategoryUUID: doc.CategoryUUID,
			CategoryName: doc.CategoryName,
			ImageURL:     doc.ImageURL,
			Status:       "Active",
			CreatedAt:    createdAtStr,
		})
	}

	products = ps.enrichStockForDTOs(ctx, products)
	return products, totalHits, nil
}

// SuggestProducts provides ultra-fast search suggestions (< 5ms) for search-as-you-type UI
func (ps *productService) SuggestProducts(ctx context.Context, q string, limit int32) ([]v1dto.ProductSuggestionDTO, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	if ps.search == nil {
		// Fallback suggestion via PostgreSQL
		list, _, err := ps.GetAllProducts(ctx, q, "", 0, 0, 0, ProductStatusActive, "product_name", "asc", 1, limit, false)
		if err != nil {
			return nil, err
		}
		suggestions := make([]v1dto.ProductSuggestionDTO, 0, len(list))
		for _, item := range list {
			suggestions = append(suggestions, v1dto.ProductSuggestionDTO{
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

	docs, _, err := ps.search.SearchDocuments(ctx, q, "product_status = 1", nil, int64(limit), 0)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Meilisearch suggestion query failed")
		return []v1dto.ProductSuggestionDTO{}, nil
	}

	suggestions := make([]v1dto.ProductSuggestionDTO, 0, len(docs))
	for _, doc := range docs {
		suggestions = append(suggestions, v1dto.ProductSuggestionDTO{
			UUID:         doc.ID,
			Name:         doc.Name,
			Slug:         doc.Slug,
			Price:        doc.Price,
			ImageURL:     doc.ImageURL,
			CategoryName: doc.CategoryName,
		})
	}

	return suggestions, nil
}

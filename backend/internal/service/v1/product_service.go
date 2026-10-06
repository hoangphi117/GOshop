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
	repo         repository.ProductRepository
	categoryRepo repository.CategoryRepository
	cache        cache.RedisCacheService
	search       *search.Client
	sf           singleflight.Group
}

func NewProductService(repo repository.ProductRepository, categoryRepo repository.CategoryRepository, redisClient *redis.Client, searchClient *search.Client) ProductService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &productService{
		repo:         repo,
		categoryRepo: categoryRepo,
		cache:        cacheService,
		search:       searchClient,
	}
}

func (ps *productService) CreateProduct(ctx context.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error) {
	categoryUuid, err := uuid.Parse(input.CategoryUUID)
	if err != nil {
		return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
	}

	category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
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
			return sqlc.CreateProductRow{}, err
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
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

	product, err := ps.repo.Create(ctx, input.Params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to create product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error) {
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
		Products []sqlc.GetProductByUUIDRow `json:"products"`
		Total    int32                      `json:"total"`
	}

	if ps.cache != nil {
		if err := ps.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Products != nil {
			return cacheData.Products, cacheData.Total, nil
		}
	}

	type fetchResult struct {
		products []sqlc.GetProductByUUIDRow
		total    int64
	}

	res, err, _ := ps.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if ps.cache != nil {
			var cd struct {
				Products []sqlc.GetProductByUUIDRow `json:"products"`
				Total    int32                      `json:"total"`
			}
			if err := ps.cache.Get(cacheKey, &cd); err == nil && cd.Products != nil {
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

		if ps.cache != nil {
			cd := struct {
				Products []sqlc.GetProductByUUIDRow `json:"products"`
				Total    int32                      `json:"total"`
			}{
				Products: listProducts,
				Total:    int32(total),
			}
			ps.cache.Set(cacheKey, cd, 5*time.Minute)
		}

		return fetchResult{products: listProducts, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	fr := res.(fetchResult)
	return fr.products, int32(fr.total), nil
}

func (ps *productService) GetProductByUUID(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	product, err := ps.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	return product, nil
}

func (ps *productService) GetProductBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
	product, err := ps.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductBySlugRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductBySlugRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	return product, nil
}

func (ps *productService) UpdateProduct(ctx context.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error) {
	if input.CategoryUUID != nil {
		categoryUuid, err := uuid.Parse(*input.CategoryUUID)
		if err != nil {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
		}

		category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
			}
			return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
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
				return sqlc.UpdateProductRow{}, err
			}
			return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
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

	product, err := ps.repo.Update(ctx, input.Params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to update product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) SoftDeleteProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	product, err := ps.repo.SoftDelete(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) RestoreProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	product, err := ps.repo.Restore(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) TrashProduct(ctx context.Context, productUUID uuid.UUID) error {
	_, err := ps.repo.Trash(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash product", err)
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
	defer tx.Rollback(ctx)

	qtx := sqlc.New(tx)
	for _, row := range rows {
		slug := fmt.Sprintf("%s-%s", utils.Slugify(row.Name), uuid.New().String()[:8])
		skuCopy := row.SKU

		_, err := qtx.CreateProduct(ctx, sqlc.CreateProductParams{
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
		return v1dto.MapListProductsToDTO(list), int64(total), nil
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
		return v1dto.MapListProductsToDTO(list), int64(total), nil
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
			price := utils.NumericToFloat(item.ProductPrice)
			img := ""
			if item.ProductImageUrl != nil {
				img = *item.ProductImageUrl
			}
			suggestions = append(suggestions, v1dto.ProductSuggestionDTO{
				UUID:         item.ProductUuid.String(),
				Name:         item.ProductName,
				Slug:         item.ProductSlug,
				Price:        price,
				ImageURL:     img,
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



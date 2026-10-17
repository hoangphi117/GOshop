package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"goshop/internal/db"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/search"

	"github.com/joho/godotenv"
)

func main() {
	rootDir := utils.MustGetwd()

	// 1. Setup minimal logger
	logger.InitLogger(logger.LoggerConfig{
		Level:    "info",
		Filename: filepath.Join(rootDir, "internal/logs/app.log"),
		IsDev:    "development",
	})

	// 2. Load .env
	_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	_ = godotenv.Load(filepath.Join(rootDir, "../.env"))

	fmt.Println("🚀 Starting GOshop Database Seeder...")

	// 3. Connect to PostgreSQL
	if err := db.InitDB("seed"); err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.DBPool.Close()

	// 4. Locate seed.sql
	possiblePaths := []string{
		filepath.Join(rootDir, "scripts/seed.sql"),
		filepath.Join(rootDir, "../scripts/seed.sql"),
		filepath.Join(rootDir, "../../scripts/seed.sql"),
		"scripts/seed.sql",
		"../scripts/seed.sql",
	}

	var seedPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			seedPath = p
			break
		}
	}

	if seedPath == "" {
		fmt.Println("❌ Could not find scripts/seed.sql in root or parent directories.")
		os.Exit(1)
	}

	fmt.Printf("📖 Reading seed file from: %s\n", seedPath)
	content, err := os.ReadFile(seedPath)
	if err != nil {
		fmt.Printf("❌ Error reading seed file: %v\n", err)
		os.Exit(1)
	}

	// 5. Execute seed SQL
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	startTime := time.Now()
	_, err = db.DBPool.Exec(ctx, string(content))
	if err != nil {
		fmt.Printf("❌ Seeding execution failed: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ Seeding completed successfully in %v!\n", duration)
	fmt.Println("📦 Seeded tables: users, categories, products, user_addresses, coupons, orders, order_items, payments, reviews, carts, stock_movements, banners, notifications, chat.")

	// 6. Sync Products to Meilisearch
	fmt.Println("🔍 Syncing products to Meilisearch...")
	meiliHost := utils.GetEnv("MEILISEARCH_HOST", "http://localhost:7700")
	meiliKey := utils.GetEnv("MEILISEARCH_MASTER_KEY", "masterKey123")
	meiliIndex := utils.GetEnv("MEILISEARCH_INDEX_PRODUCTS", "products")

	meiliClient, err := search.NewClient(meiliHost, meiliKey, meiliIndex)
	if err != nil {
		fmt.Printf("⚠️ Meilisearch client initialization warning: %v\n", err)
		return
	}

	if err := meiliClient.SetupIndex(ctx); err != nil {
		fmt.Printf("⚠️ Failed to setup Meilisearch index: %v\n", err)
	}

	// Fetch all active/non-deleted products from PostgreSQL
	rows, err := db.DBPool.Query(ctx, `
		SELECT 
			p.product_uuid, p.product_id, p.product_name, p.product_slug, 
			COALESCE(p.product_description, ''), p.product_price, p.product_stock, 
			p.product_sku, c.category_uuid, c.category_name, 
			COALESCE(p.product_image_url, ''), p.product_status,
			EXTRACT(EPOCH FROM p.product_created_at)::bigint
		FROM products p
		JOIN categories c ON c.category_id = p.product_category_id
		WHERE p.product_deleted_at IS NULL
	`)
	if err != nil {
		fmt.Printf("⚠️ Failed to fetch products for Meilisearch sync: %v\n", err)
		return
	}
	defer rows.Close()

	var documents []search.ProductDocument
	for rows.Next() {
		var doc search.ProductDocument
		var pUUID, cUUID string
		if err := rows.Scan(
			&pUUID, &doc.ProductID, &doc.Name, &doc.Slug,
			&doc.Description, &doc.Price, &doc.Stock,
			&doc.SKU, &cUUID, &doc.CategoryName,
			&doc.ImageURL, &doc.Status, &doc.CreatedAt,
		); err != nil {
			fmt.Printf("⚠️ Error scanning product row: %v\n", err)
			continue
		}
		doc.ID = pUUID
		doc.CategoryUUID = cUUID
		documents = append(documents, doc)
	}

	if len(documents) > 0 {
		// Batch indexing (1000 docs per chunk)
		batchSize := 1000
		for i := 0; i < len(documents); i += batchSize {
			end := i + batchSize
			if end > len(documents) {
				end = len(documents)
			}
			chunk := documents[i:end]
			if _, err := meiliClient.IndexProducts(ctx, chunk); err != nil {
				fmt.Printf("⚠️ Failed to index batch [%d..%d]: %v\n", i, end, err)
			} else {
				fmt.Printf("   Indexed products %d..%d to Meilisearch\n", i+1, end)
			}
		}
		fmt.Printf("🎯 Successfully synced %d products to Meilisearch with typo tolerance!\n", len(documents))
	}
}


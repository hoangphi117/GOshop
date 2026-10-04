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
	if err := db.InitDB(); err != nil {
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
}

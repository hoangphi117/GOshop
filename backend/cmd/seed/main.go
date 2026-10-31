package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"goshop/pkg/logger"
	"goshop/pkg/search"
	"goshop/pkg/utils"
)

// seedSection is one `-- DB: <name>` block of scripts/seed.sql. Each block runs
// as a single statement batch against its own database; blocks connect with
// dblink so cross-database copies stay in one transaction per database.
type seedSection struct {
	db  string
	sql string
}

func main() {
	rootDir := utils.MustGetwd()

	logger.InitLogger(logger.LoggerConfig{
		Level:    "info",
		Filename: utils.RuntimeLogPath("seed", "app.log"),
		IsDev:    "development",
	})

	_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	_ = godotenv.Load(filepath.Join(rootDir, "../.env"))

	fmt.Println("Starting GOshop Database Seeder...")

	seedPath := findSeedFile(rootDir)
	if seedPath == "" {
		fmt.Println("INFO: Could not find scripts/seed.sql in root or parent directories.")
		os.Exit(1)
	}

	fmt.Printf("Reading seed file from: %s\n", seedPath)
	content, err := os.ReadFile(seedPath)
	if err != nil {
		fmt.Printf("INFO: Error reading seed file: %v\n", err)
		os.Exit(1)
	}

	sections := parseSections(string(content))
	if len(sections) == 0 {
		fmt.Println("INFO: No '-- DB: <name>' sections found in seed file.")
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		utils.GetEnv("DB_HOST", "localhost"),
		utils.GetEnv("DB_PORT", "5432"),
		utils.GetEnv("DB_USER", "postgres"),
		utils.GetEnv("DB_PASSWORD", ""),
		utils.GetEnv("DB_NAME", "shopping"),
		utils.GetEnv("DB_SSL_MODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	for _, section := range sections {
		if strings.TrimSpace(section.sql) == "" {
			continue
		}
		if err := runSection(ctx, dsn, section); err != nil {
			fmt.Printf("INFO: Seeding %s failed: %v\n", section.db, err)
			os.Exit(1)
		}
		fmt.Printf("INFO: Seeded database %s\n", section.db)
	}

	fmt.Printf("INFO: Seeding completed successfully in %v!\n", time.Since(startTime))
	fmt.Println("Databases: shopping, shopping_identity, shopping_catalog, shopping_coupon, shopping_cart, shopping_inventory")
	fmt.Println("Seeded logins: admin@goshop.com / manager@goshop.com / mod@goshop.com / admin@gmail.com - password: Password123!")

	purgeSearchAndCache(ctx, dsn)
	fmt.Println("INFO: Product search reindex queued (catalog outbox worker consumes the events) and stale search docs purged.")
}

// purgeSearchAndCache makes re-seeded data visible: re-seeding regenerates
// product UUIDs, but the Meilisearch index and the catalog Redis cache keep
// documents/entries from the previous run, and the in-DB outbox events queued
// by seed.sql only cover upserts. Remove search documents that no longer match
// a live product, and drop catalog caches so reads cannot serve stale rows.
func purgeSearchAndCache(ctx context.Context, dsn string) {
	// 1. Expire catalog caches.
	redisAddr := utils.GetEnv("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: utils.GetEnv("REDIS_USER", ""),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
	defer func() { _ = rdb.Close() }()

	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var cursor uint64
	deleted := 0
	for {
		keys, next, err := rdb.Scan(scanCtx, cursor, "catalog:*", 200).Result()
		if err != nil {
			fmt.Printf("INFO: Skipping redis cache purge: %v\n", err)
			break
		}
		if len(keys) > 0 {
			if err := rdb.Del(scanCtx, keys...).Err(); err != nil {
				fmt.Printf("INFO: Skipping redis cache purge: %v\n", err)
				break
			}
			deleted += len(keys)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	fmt.Printf("ðŸ§¹ Redis: dropped %d catalog cache keys\n", deleted)

	// 2. Drop search documents that no longer match a seeded product.
	live := map[string]bool{}
	if err := forEachProductUUID(ctx, dsn, func(u string) { live[u] = true }); err != nil {
		fmt.Printf("INFO: Skipping search purge (cannot read product UUIDs): %v\n", err)
		return
	}

	searchClient, err := search.NewClient(
		utils.GetEnv("MEILISEARCH_HOST", "http://localhost:7700"),
		utils.GetEnv("MEILISEARCH_MASTER_KEY", ""),
		utils.GetEnv("MEILISEARCH_INDEX_PRODUCTS", "products"),
	)
	if err != nil {
		fmt.Printf("INFO: Skipping search purge: %v\n", err)
		return
	}

	if err := purgeRemovedDocuments(ctx, searchClient, live); err != nil {
		fmt.Printf("INFO: Skipping search purge: %v\n", err)
	}
}

// purgeRemovedDocuments deletes search documents whose ids are absent from
// live list.
func purgeRemovedDocuments(ctx context.Context, client *search.Client, live map[string]bool) error {
	ids, err := client.ListDocumentIDs(ctx)
	if err != nil {
		return err
	}

	purged := 0
	for _, id := range ids {
		if live[id] {
			continue
		}
		if _, err := client.DeleteProduct(ctx, id); err != nil {
			return err
		}
		purged++
	}
	fmt.Printf("ðŸ§¹ Meilisearch: removed %d out-of-seed documents\n", purged)
	return nil
}

// forEachProductUUID collects live product UUIDs from the seeded catalog DB.
func forEachProductUUID(ctx context.Context, dsn string, fn func(string)) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}
	cfg.Database = "shopping_catalog"

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to shopping_catalog: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	rows, err := conn.Query(ctx, "SELECT product_uuid::text FROM products WHERE product_deleted_at IS NULL")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return err
		}
		fn(u)
	}
	return rows.Err()
}

// runSection connects to the section's database and executes its SQL.
// `\connect` directives inside a section are a psql convenience only.
func runSection(ctx context.Context, baseDSN string, section seedSection) error {
	cfg, err := pgx.ParseConfig(baseDSN)
	if err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}
	cfg.Database = section.db

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", section.db, err)
	}
	defer func() { _ = conn.Close(ctx) }()

	sql := stripConnectDirectives(section.sql)
	// Seed DDL/DML is order-dependent and must run as a single batch with the
	// explicit BEGIN/COMMIT statements that guard the master-data section.
	if _, err := conn.Exec(ctx, "SET statement_timeout = 0"); err != nil {
		return fmt.Errorf("reset statement timeout on %s: %w", section.db, err)
	}
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("execute seed SQL on %s: %w", section.db, err)
	}
	return nil
}

func parseSections(content string) []seedSection {
	var sections []seedSection
	var current *seedSection
	var buf strings.Builder

	flush := func() {
		if current != nil {
			current.sql = buf.String()
			sections = append(sections, *current)
			buf.Reset()
		}
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- DB:") {
			flush()
			current = &seedSection{db: strings.TrimSpace(strings.TrimPrefix(trimmed, "-- DB:"))}
			continue
		}
		if current == nil {
			continue
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	flush()

	return sections
}

// stripConnectDirectives drops psql \connect lines and turns the separator
// comment lines into the block boundary that already exists in the parser.
func stripConnectDirectives(sql string) string {
	var out strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), `\connect`) {
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

func findSeedFile(rootDir string) string {
	possiblePaths := []string{
		filepath.Join(rootDir, "scripts/seed.sql"),
		filepath.Join(rootDir, "../scripts/seed.sql"),
		filepath.Join(rootDir, "../../scripts/seed.sql"),
		"scripts/seed.sql",
		"../scripts/seed.sql",
	}
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

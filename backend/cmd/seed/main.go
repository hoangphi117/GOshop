package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goshop/internal/utils"
	"goshop/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
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
		Filename: filepath.Join(rootDir, "internal/logs/app.log"),
		IsDev:    "development",
	})

	_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	_ = godotenv.Load(filepath.Join(rootDir, "../.env"))

	fmt.Println("🚀 Starting GOshop Database Seeder...")

	seedPath := findSeedFile(rootDir)
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

	sections := parseSections(string(content))
	if len(sections) == 0 {
		fmt.Println("❌ No '-- DB: <name>' sections found in seed file.")
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
			fmt.Printf("❌ Seeding %s failed: %v\n", section.db, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Seeded database %s\n", section.db)
	}

	fmt.Printf("✅ Seeding completed successfully in %v!\n", time.Since(startTime))
	fmt.Println("📦 Databases: shopping, shopping_identity, shopping_catalog, shopping_coupon, shopping_cart, shopping_inventory")
	fmt.Println("🔐 Seeded logins: admin@goshop.com / manager@goshop.com / mod@goshop.com / admin@gmail.com — password: Password123!")
	fmt.Println("🔍 Product search is synced by the catalog service outbox worker (no manual Meilisearch step).")
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

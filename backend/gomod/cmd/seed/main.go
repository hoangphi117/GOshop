package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"goshop/shared/config"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/search"
	"goshop/shared/pkg/utils"
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
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=public",
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
	redisOpt, err := config.NewOptionalRedisOptions()
	if err != nil || redisOpt == nil {
		logger.Log.Warn().Msg("Redis not configured; skipping cache purge")
		return
	}
	rdb := redis.NewClient(redisOpt)
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
	// dblink opens its own libpq connection with no inherited credentials, so
	// `dblink('dbname=shopping_catalog', ...)` only works where pg_hba trusts
	// local socket users. On a managed server (Supabase) it fails with
	// "password or GSSAPI delegated credentials required", and the pooler does
	// not expose db_host/user as GUCs to rebuild the conninfo in SQL either.
	// Substituting here is the one place that has both the plaintext DSN and the
	// SQL text.
	sql = expandSeedVars(sql, conninfoFor(cfg, baseDSN))
	// Protective: an unexpanded {{...}} means the passthrough SQL will still
	// contain a placeholder and fail with a confusing syntax error.
	if strings.Contains(sql, "{{") {
		return fmt.Errorf("unexpanded seed placeholder on %s: %s",
			section.db, sql[strings.Index(sql, "{{"):min(strings.Index(sql, "{{")+40, len(sql))])
	}
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

// conninfoFor renders a libpq key/value conninfo for a sibling database on the
// same server, for dblink callers.
//
// search_path must be explicit: a transaction-mode pooler opens every backend
// with `SET search_path = pg_catalog`, so a dblink connection that does not ask
// for `public` sees none of the tables it is meant to read.
func conninfoFor(cfg *pgx.ConnConfig, baseDSN string) map[string]string {
	sslmode := "disable"
	if strings.Contains(baseDSN, "sslmode=require") {
		sslmode = "require"
	}

	// dblink opens its own libpq connection, bypassing pgx, so it does not get
	// the DSN the seeder connected with. Reuse the outer DSN's host and port:
	// on Supabase the direct host (db.<ref>.supabase.co) is AAAA-only and
	// therefore unreachable from an IPv4-only network, so the pooler host is
	// the only address that resolves. DB_DIRECT_HOST lets a caller force the
	// session port (5432); transaction-mode 6543 rejects the nested hop with
	// "password or GSSAPI delegated credentials required".
	dblinkHost := cfg.Host
	dblinkPort := int(cfg.Port)
	if h := os.Getenv("DB_DIRECT_HOST"); h != "" {
		dblinkHost = h
	}
	if p := os.Getenv("DB_DIRECT_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			dblinkPort = n
		}
	}

	out := map[string]string{
		"db_host": dblinkHost,
		"db_port": strconv.Itoa(dblinkPort),
		"db_user": cfg.User,
		// libpq conninfo is whitespace-separated, so a password with a space or
		// quote would truncate the rest of the string.
		"db_password": quoteConninfoValue(cfg.Password),
	}
	servers := map[string]string{
		"db_identity":  "shopping_identity",
		"db_catalog":   "shopping_catalog",
		"db_order":     "shopping_order",
		"db_inventory": "shopping_inventory",
	}
	for key, db := range servers {
		out[key] = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dblinkHost, dblinkPort, cfg.User, quoteConninfoValue(cfg.Password), db, sslmode)
	}
	// dblink's conninfo parser accepts `search_path`, but some libpq builds
	// ignore it; the explicit SET inside the dblink session is then the only
	// thing that puts `public` on the path.
	out["db_search_path"] = "public"
	out["db_search_path_order"] = "orders, public"
	return out
}

// quoteConninfoValue wraps a value in single quotes for libpq conninfo syntax
// when it contains characters that would otherwise split the string.
func quoteConninfoValue(v string) string {
	if !strings.ContainsAny(v, " '\\") {
		return v
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

func expandSeedVars(sql string, vars map[string]string) string {
	for k, v := range vars {
		sql = strings.ReplaceAll(sql, "{{"+k+"}}", v)
	}
	return sql
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

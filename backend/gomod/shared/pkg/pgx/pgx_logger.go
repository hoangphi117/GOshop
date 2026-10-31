package pgx

import (
	"context"
	"fmt"
	"goshop/shared/pkg/logger"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"time"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

type PgxZerologTracer struct {
	Logger         zerolog.Logger
	SlowQueryLimit time.Duration
}

type QueryInfo struct {
	QueryName   string
	Operation   string
	CleanSQL    string
	OriginalSQL string
}

var (
	sqlcNameRegex = regexp.MustCompile(`-- name:\s*(\w+)\s*:(\w+)`)
	spaceRegex    = regexp.MustCompile(`\s+`)
	commentRegex  = regexp.MustCompile(`-- [^\r\n]*`)
)

func parseSQL(sql string) QueryInfo {
	info := QueryInfo{
		OriginalSQL: sql,
	}

	if sqlcMatch := sqlcNameRegex.FindStringSubmatch(sql); len(sqlcMatch) == 3 {
		info.QueryName = sqlcMatch[1]
		info.Operation = strings.ToUpper(sqlcMatch[2])
	}

	cleanSQL := commentRegex.ReplaceAllString(sql, "")
	cleanSQL = spaceRegex.ReplaceAllString(cleanSQL, " ")
	cleanSQL = strings.TrimSpace(cleanSQL)
	info.CleanSQL = cleanSQL

	return info
}

func formatArgs(arg any) string {

	val := reflect.ValueOf(arg)

	if arg == nil || (val.Kind() == reflect.Pointer && val.IsNil()) {
		return "NULL"
	}

	if val.Kind() == reflect.Pointer {
		arg = val.Elem().Interface()
	}

	switch v := arg.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format(time.RFC3339))
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''"))
	}
}

func replacePlaceholders(sql string, args []any) string {
	for i, arg := range args {
		placeholder := "$" + strconv.Itoa(i+1)
		sql = strings.Replace(sql, placeholder, formatArgs(arg), 1)
	}
	return sql
}

func (t *PgxZerologTracer) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {

	sql, _ := data["sql"].(string)
	args, _ := data["args"].([]any)
	duration, _ := data["time"].(time.Duration)

	safeArgs := make([]any, len(args))
	for i, arg := range args {
		if b, ok := arg.([]byte); ok {
			safeArgs[i] = "[" + string(rune(len(b))) + " bytes]"
		} else {
			safeArgs[i] = arg
		}
	}

	queryInfo := parseSQL(sql)

	var finalSQL string
	if len(args) > 0 {
		finalSQL = replacePlaceholders(queryInfo.CleanSQL, args)
	} else {
		finalSQL = queryInfo.CleanSQL
	}

	baseLogger := t.Logger.With().
		Str("trace_id", logger.GetTraceId(ctx)).
		Dur("duration", duration).
		Str("sql_original", queryInfo.OriginalSQL).
		Str("sql", finalSQL).
		Str("sql_name", queryInfo.QueryName).
		Str("sql_operation", queryInfo.Operation).
		Interface("args", safeArgs)

	logger := baseLogger.Logger()

	if msg == "Query" && duration > t.SlowQueryLimit {
		logger.Warn().Str("event", "Slow query").Msg("Slow SQL Query")
		return
	}

	if msg == "Query" {
		logger.Info().Str("event", "SQL query").Msg("SQL Query")
		return
	}
}

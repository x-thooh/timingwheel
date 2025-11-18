package database

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/x-thooh/delay/pkg/log"
)

type Hooks struct {
	Logger log.Logger
}

type ctxKey string

const beginKey ctxKey = "begin"

// --- 参数格式化 ---
func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return fmt.Sprintf("'%v'", v)
	case []byte:
		return fmt.Sprintf("'%v'", string(v))
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format("2006-01-02 15:04:05"))
	case nil:
		return "NULL"
	case []int:
		return joinInts(v)
	case []int64:
		return joinInts64(v)
	case []string:
		return joinStrings(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func joinInts(arr []int) string {
	s := make([]string, len(arr))
	for i, num := range arr {
		s[i] = fmt.Sprintf("%d", num)
	}
	return strings.Join(s, ",")
}

func joinInts64(arr []int64) string {
	s := make([]string, len(arr))
	for i, num := range arr {
		s[i] = fmt.Sprintf("%d", num)
	}
	return strings.Join(s, ",")
}

func joinStrings(arr []string) string {
	s := make([]string, len(arr))
	for i, str := range arr {
		s[i] = fmt.Sprintf("'%s'", str)
	}
	return strings.Join(s, ",")
}

// --- Placeholder 替换 ---
func replacePlaceholders(query string, args ...interface{}) string {
	var b strings.Builder
	argIndex := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' && argIndex < len(args) {
			b.WriteString(formatArg(args[argIndex]))
			argIndex++
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

func oneLineSQL(sql string) string {
	sql = strings.ReplaceAll(sql, "\n", " ")
	sql = strings.Join(strings.Fields(sql), " ")
	return sql
}

func (h *Hooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	finalSQL := oneLineSQL(replacePlaceholders(query, args...))
	ctx = context.WithValue(ctx, "sql", finalSQL)
	return context.WithValue(ctx, beginKey, time.Now()), nil
}

func (h *Hooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	begin, ok := ctx.Value(beginKey).(time.Time)
	sql, _ := ctx.Value("sql").(string)

	if ok {
		// 使用 slog 记录
		h.Logger.Debug(
			ctx,
			"Executing SQL",
			slog.String("duration", time.Since(begin).String()),
			slog.String("sql", sql),
		)
	}
	return ctx, nil
}

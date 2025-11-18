package xslog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

// ColorHandler 实现 slog.Handler，支持 SQL 高亮
type ColorHandler struct {
	out     io.Writer
	opts    *slog.HandlerOptions
	isColor bool
	attrs   []slog.Attr
}

// NewANSIColorHandler 创建一个 Handler
func NewANSIColorHandler(w io.Writer, opts *slog.HandlerOptions, isColor bool) slog.Handler {
	return &ColorHandler{
		out:     w,
		opts:    opts,
		isColor: isColor,
	}
}

// Enabled 决定是否输出日志
func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts != nil && h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}
	return true
}

// Handle 处理日志输出
func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	// 时间 + 级别 + 消息
	fmt.Fprintf(&b, "[%s] [%s] %s",
		r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		strings.ToUpper(r.Level.String()),
		r.Message,
	)

	// --- 输出 logger 固定属性（比如 trace_id） ---
	for _, attr := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", attr.Key, attr.Value)
	}

	// 遍历属性
	r.Attrs(func(attr slog.Attr) bool {
		val := attr.Value.String()
		if h.isColor && attr.Key == "sql" {
			val = colorSQL(val)
		}
		fmt.Fprintf(&b, " %s=%v", attr.Key, val)
		return true
	})

	b.WriteString("\n")

	_, err := h.out.Write([]byte(b.String()))
	return err
}

// WithAttrs 追加属性
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// 拷贝已有属性，避免修改原 handler
	newH := *h
	newH.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &newH
}

// WithGroup 分组（简化返回自己）
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return h
}

// --- SQL 高亮 ---
func colorSQL(sql string) string {
	// 高亮样式
	const yellow = "\033[33m"
	const reset = "\033[0m"

	// SQL 关键字
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE",
		"FROM", "WHERE", "VALUES", "SET",
		"JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "OUTER JOIN",
		"ON", "AND", "OR", "IN", "AS",
		"LIMIT", "GROUP BY", "ORDER BY", "HAVING", "DISTINCT",
		"IGNORE", "NOT", "NULL", "IS", "BETWEEN", "EXISTS",
	}

	// 构造正则：匹配完整单词（\b 边界），忽略大小写
	pattern := `(?i)\b(` + strings.Join(keywords, `|`) + `)\b`
	re := regexp.MustCompile(pattern)

	// 用函数替换，保留原大小写
	sql = re.ReplaceAllStringFunc(sql, func(s string) string {
		return yellow + s + reset
	})

	// 高亮常见符号
	symbols := []string{"=", ">", "<", ">=", "<=", "<>", ","}
	for _, sym := range symbols {
		sql = strings.ReplaceAll(sql, sym, "\033[36m"+sym+reset) // 青色符号
	}

	return sql
}

package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// ANSI color codes.
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
	white   = "\033[97m"
)

// ColorTextHandler writes human-readable, optionally colorized log lines to w.
type ColorTextHandler struct {
	w      io.Writer
	level  slog.Leveler
	color  bool
	mu     sync.Mutex
	attrs  []slog.Attr
	groups []string
}

func NewColorTextHandler(w io.Writer, opts *slog.HandlerOptions) *ColorTextHandler {
	var level slog.Leveler = slog.LevelInfo
	if opts != nil && opts.Level != nil {
		level = opts.Level
	}
	return &ColorTextHandler{
		w:     w,
		level: level,
		color: colorsEnabled(),
	}
}

func colorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func (h *ColorTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ColorTextHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	ts := r.Time.Format("15:04:05.000")
	level := strings.ToUpper(r.Level.String())

	var b strings.Builder
	if h.color {
		b.WriteString(gray + ts + reset)
		b.WriteString("  ")
		b.WriteString(levelColor(r.Level, level))
		b.WriteString("  ")
		b.WriteString(bold + white + r.Message + reset)
	} else {
		fmt.Fprintf(&b, "%s  %-5s  %s", ts, level, r.Message)
	}

	attrs := collectAttrs(h.attrs, r)
	if len(attrs) > 0 {
		b.WriteString("\n              ")
		b.WriteString(formatAttrs(attrs, h.color))
	}

	b.WriteByte('\n')
	_, err := h.w.Write([]byte(b.String()))
	return err
}

func (h *ColorTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *ColorTextHandler) WithGroup(name string) slog.Handler {
	next := *h
	next.groups = append(append([]string{}, h.groups...), name)
	return &next
}

func collectAttrs(base []slog.Attr, r slog.Record) []slog.Attr {
	attrs := append([]slog.Attr{}, base...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	return attrs
}

func levelColor(l slog.Level, label string) string {
	switch {
	case l >= slog.LevelError:
		return bold + red + label + reset
	case l >= slog.LevelWarn:
		return yellow + label + reset
	case l >= slog.LevelInfo:
		return green + label + reset
	default:
		return cyan + label + reset
	}
}

func formatAttrs(attrs []slog.Attr, color bool) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		key := a.Key
		val := formatValue(a.Value)
		if color {
			parts = append(parts, attrColor(key, val))
		} else {
			parts = append(parts, key+"="+val)
		}
	}
	return strings.Join(parts, "  ")
}

func attrColor(key, val string) string {
	switch key {
	case "event":
		return blue + key + "=" + val + reset
	case "room", "turn":
		return magenta + key + "=" + val + reset
	case "ttfb_ms", "e2e_ms", "duration_ms", "buffered_ms", "queue_ms":
		return yellow + key + "=" + val + reset
	case "text", "text_preview":
		return dim + cyan + key + `="` + val + `"` + reset
	case "error":
		return red + key + "=" + val + reset
	default:
		return gray + key + "=" + val + reset
	}
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, " \t\n\"") {
			return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
		}
		return s
	case slog.KindInt64:
		return fmt.Sprintf("%d", v.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", v.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%g", v.Float64())
	case slog.KindBool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case slog.KindDuration:
		return v.Duration().Round(time.Millisecond).String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		return fmt.Sprintf("%v", v.Any())
	default:
		return v.String()
	}
}

package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// Redacted is what a secret looks like in a log line.
const Redacted = "***"

// secretKeys matches attribute keys whose value must never be logged. The
// config service already redacts on the way out (ADR-0010); this is the second
// belt, because a log line is written from a hundred places and only one of
// them has to be careless.
var secretKeys = []string{
	"password", "secret", "token", "apikey", "api_key", "apitoken",
	"authorization", "credential", "private_key", "passphrase", "cookie",
}

// secretValues matches values that look like a credential regardless of the key
// they were logged under.
var secretValues = regexp.MustCompile(
	`(?i)\b(?:aos_[0-9a-f]{16,}|sk-[A-Za-z0-9_\-]{16,}|gh[pousr]_[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._\-]{16,})`,
)

type redacting struct{ next slog.Handler }

// redactHandler drops any attribute whose key matches a secret pattern, and any
// value that matches a known token shape.
func redactHandler(next slog.Handler) slog.Handler { return redacting{next: next} }

func (h redacting) Enabled(ctx context.Context, l slog.Level) bool { return h.next.Enabled(ctx, l) }

func (h redacting) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, secretValues.ReplaceAllString(r.Message, Redacted), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, out)
}

func (h redacting) WithAttrs(attrs []slog.Attr) slog.Handler {
	cleaned := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		cleaned = append(cleaned, redactAttr(a))
	}
	return redacting{next: h.next.WithAttrs(cleaned)}
}

func (h redacting) WithGroup(name string) slog.Handler {
	return redacting{next: h.next.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	if isSecretKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		group := v.Group()
		cleaned := make([]slog.Attr, 0, len(group))
		for _, g := range group {
			cleaned = append(cleaned, redactAttr(g))
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(cleaned...)}
	}
	if v.Kind() == slog.KindString {
		if s := v.String(); secretValues.MatchString(s) {
			return slog.String(a.Key, secretValues.ReplaceAllString(s, Redacted))
		}
	}
	return a
}

func isSecretKey(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, s := range secretKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// Package env resolves settings in layers.
//
// Precedence, highest first: an explicit override layer (flags), the process
// environment, then the built-in default. Every key is written without the
// product prefix — "SERVER_PORT" reads AOS_SERVER_PORT — so that renaming the
// product does not touch a single call site (ADR-0000).
package env

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/build"
)

// Source is one layer of resolution.
type Source interface {
	Lookup(key string) (string, bool)
}

// Resolver reads a key from its layers, first hit wins.
type Resolver struct {
	layers []Source
}

// New builds a resolver from layers ordered highest precedence first.
func New(layers ...Source) *Resolver { return &Resolver{layers: layers} }

// Default is the resolver used by production code: the process environment
// only. Tests build their own with Map layers instead of mutating os.Environ.
func Default() *Resolver { return New(OS()) }

// Key returns the full environment variable name for a bare key.
func Key(name string) string { return build.EnvPrefix + "_" + strings.ToUpper(name) }

type osSource struct{}

func (osSource) Lookup(key string) (string, bool) { return os.LookupEnv(Key(key)) }

// OS is the process environment layer.
func OS() Source { return osSource{} }

type mapSource map[string]string

func (m mapSource) Lookup(key string) (string, bool) {
	v, ok := m[strings.ToUpper(key)]
	return v, ok
}

// Map is a layer backed by an in-memory map, keyed without the prefix.
func Map(m map[string]string) Source {
	up := make(mapSource, len(m))
	for k, v := range m {
		up[strings.ToUpper(k)] = v
	}
	return up
}

func (r *Resolver) lookup(key string) (string, bool) {
	for _, l := range r.layers {
		if v, ok := l.Lookup(key); ok && v != "" {
			return v, true
		}
	}
	return "", false
}

// String resolves a string setting.
func (r *Resolver) String(key, def string) string {
	if v, ok := r.lookup(key); ok {
		return v
	}
	return def
}

// Int resolves an integer setting, falling back to def when absent or invalid.
// An invalid value is not an error: a daemon must boot with a sane default
// rather than refuse to start because of a typo in a shell profile.
func (r *Resolver) Int(key string, def int) int {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Bool resolves a boolean setting. "1", "true", "yes" and "on" are true.
func (r *Resolver) Bool(key string, def bool) bool {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// Duration resolves a duration setting written in Go syntax ("15m", "30s").
func (r *Resolver) Duration(key string, def time.Duration) time.Duration {
	v, ok := r.lookup(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

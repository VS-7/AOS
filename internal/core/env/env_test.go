package env_test

import (
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/env"
)

func TestKeyCarriesBrandPrefix(t *testing.T) {
	if got, want := env.Key("server_port"), build.EnvPrefix+"_SERVER_PORT"; got != want {
		t.Fatalf("Key = %q, want %q", got, want)
	}
}

func TestLayersResolveHighestFirst(t *testing.T) {
	r := env.New(
		env.Map(map[string]string{"LOG_LEVEL": "debug"}),
		env.Map(map[string]string{"LOG_LEVEL": "info", "SERVER_PORT": "1234"}),
	)
	if got := r.String("LOG_LEVEL", "warn"); got != "debug" {
		t.Errorf("LOG_LEVEL = %q, want debug", got)
	}
	if got := r.Int("SERVER_PORT", 5326); got != 1234 {
		t.Errorf("SERVER_PORT = %d, want 1234", got)
	}
	if got := r.String("MISSING", "fallback"); got != "fallback" {
		t.Errorf("MISSING = %q, want fallback", got)
	}
}

func TestEmptyValueFallsThrough(t *testing.T) {
	// An exported-but-empty variable is how a shell profile accidentally
	// blanks a setting; it must not beat the default.
	r := env.New(env.Map(map[string]string{"LOG_LEVEL": ""}))
	if got := r.String("LOG_LEVEL", "info"); got != "info" {
		t.Fatalf("LOG_LEVEL = %q, want info", got)
	}
}

func TestInvalidValuesFallBackInsteadOfFailing(t *testing.T) {
	r := env.New(env.Map(map[string]string{
		"JOBS_CONCURRENCY": "many",
		"JOBS_TICK":        "soon",
		"FSYNC":            "maybe",
	}))
	if got := r.Int(env.KeyJobsConcurrency, env.DefaultJobsConcurrency); got != env.DefaultJobsConcurrency {
		t.Errorf("concurrency = %d", got)
	}
	if got := r.Duration(env.KeyJobsTick, env.DefaultJobsTick); got != env.DefaultJobsTick {
		t.Errorf("tick = %v", got)
	}
	if got := r.Bool(env.KeyFsync, true); !got {
		t.Error("invalid bool should keep the default")
	}
}

func TestBoolAndDurationParsing(t *testing.T) {
	r := env.New(env.Map(map[string]string{
		"A": "yes", "B": "off", "C": "90s",
	}))
	if !r.Bool("A", false) {
		t.Error("yes should be true")
	}
	if r.Bool("B", true) {
		t.Error("off should be false")
	}
	if got := r.Duration("C", time.Minute); got != 90*time.Second {
		t.Errorf("duration = %v", got)
	}
}

func TestProductionIsTheDefaultMode(t *testing.T) {
	// The original shipped with development mode hardcoded on (defect #12).
	if !env.New().IsProduction() {
		t.Fatal("an unset environment must resolve to production")
	}
	if env.New(env.Map(map[string]string{"ENV": "development"})).IsProduction() {
		t.Fatal("ENV=development must not be production")
	}
}

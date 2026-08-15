package clix

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/transport/clix/format"
)

// The global flags. They are what separates this CLI from a conventional one:
// no ordinary CLI paginates its output by tokens, filters it by path, or emits
// its own JSON Schema. Here the primary consumer is a model with a finite
// context window.
const (
	flagFormat      = "format"
	flagFilterOut   = "filter-output"
	flagTokenLimit  = "token-limit"
	flagTokenOffset = "token-offset"
	flagTokenCount  = "token-count"
	flagSchema      = "schema"
	flagBaseURL     = "base-url"
	flagToken       = "token"
	flagWorkspace   = "workspace"
	flagAgent       = "agent"
)

func addGlobalFlags(fs *pflag.FlagSet) {
	fs.String(flagFormat, "", "output format: "+format.Join(format.All)+
		" (default toon on a terminal, json otherwise)")
	fs.String(flagFilterOut, "", "select paths out of the result: foo,bar.baz,a[0,3]")
	fs.Int(flagTokenLimit, 0, "cut the output at this many tokens, on a line boundary")
	fs.Int(flagTokenOffset, 0, "skip this many tokens before printing — the next page")
	fs.Bool(flagTokenCount, false, "print the token count of the full result instead of the result")
	fs.Bool(flagSchema, false, "print the JSON Schema of the command instead of running it")
}

// addTransportFlags declares the flags that point a command at a remote daemon.
// They are absent from commands marked Local, which never talk to an API.
func addTransportFlags(fs *pflag.FlagSet) {
	fs.String(flagBaseURL, "", "remote API base URL for this execution ["+env.Key(env.KeyServerHost)+"]")
	fs.String(flagToken, "", "API token used as Bearer authorization for this execution ["+env.Key(env.KeyToken)+"]")
	fs.String(flagWorkspace, "", "workspace id for this execution")
	fs.String(flagAgent, "", "agent id (slug), sent as X-Agent-ID")
}

// resolveOutput reads the global flags into rendering options.
//
// Without a terminal the consumer is a program, and JSON is more predictable to
// parse than TOON. The original uses TOON either way; this is the one place
// where we diverge on output, and only in the default.
func resolveOutput(cmd *cobra.Command, cfg Config) format.Options {
	opts := format.Options{Format: format.TOON}
	if !cfg.IsTTY() {
		opts.Format = format.JSON
	}
	if raw, err := cmd.Flags().GetString(flagFormat); err == nil && raw != "" {
		if f, perr := format.Parse(raw); perr == nil {
			opts.Format = f
		}
	}
	opts.Filter, _ = cmd.Flags().GetString(flagFilterOut)
	opts.TokenLimit, _ = cmd.Flags().GetInt(flagTokenLimit)
	opts.TokenOffset, _ = cmd.Flags().GetInt(flagTokenOffset)
	return opts
}

func schemaOnly(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool(flagSchema)
	return err == nil && v
}

func tokenCountOnly(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool(flagTokenCount)
	return err == nil && v
}

package clix

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

const (
	flagJSON   = "json"
	flagSet    = "set"
	flagReason = "reason"
)

// AttachDaemon adds one cobra command per entry of the daemon's published
// surface manifest to root — the mechanism this binary's own package doc
// promises: "the command tree for those arrives with the published surface
// manifest". wailsvc.Caller is reused rather than redeclared here because it
// is already exactly this port, proven by *daemonclient.Client satisfying it
// for the desktop.
//
// It is best-effort: a daemon that is not running, not reachable, or not yet
// authenticated against leaves root exactly as NewRoot built it. That matters
// most for the one command whose entire job is to start the daemon this would
// otherwise depend on — `aos gateway start` must work with nothing attached.
func AttachDaemon(ctx context.Context, root *cobra.Command, client wailsvc.Caller, cfg Config) error {
	infos, err := client.Commands(ctx)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return errNoPublishedSurface()
	}

	groups := map[string]*cobra.Command{}
	for _, existing := range root.Commands() {
		groups[existing.Name()] = existing
	}

	for _, info := range infos {
		g, ok := groups[info.Group]
		if !ok {
			g = &cobra.Command{Use: info.Group, Short: info.Group + " commands"}
			root.AddCommand(g)
			groups[info.Group] = g
		}
		// A group this binary already builds locally keeps its own commands.
		// `gateway` is the whole of that case: it is linked in here because
		// supervising a process cannot be delegated to the process being
		// supervised, and the daemon publishes it too — so `aos gateway
		// --help` listed start, stop, status and restart twice, and the
		// second of each pair reached the daemon to ask it to start itself.
		if existing := findSubcommand(g, info.Name); existing != nil {
			continue
		}
		g.AddCommand(buildDaemonCommand(info, client, cfg))
	}
	return nil
}

// findSubcommand looks a name up among a group's own commands, aliases
// included.
func findSubcommand(group *cobra.Command, name string) *cobra.Command {
	for _, c := range group.Commands() {
		if c.Name() == name {
			return c
		}
		for _, alias := range c.Aliases {
			if alias == name {
				return c
			}
		}
	}
	return nil
}

// ExplainMissingTree turns cobra's "unknown command" into the answer the
// person actually needs when the command tree could not be fetched.
//
// The tree is not compiled into this binary: it arrives from a running,
// authenticated daemon. When that fetch fails — nothing listening, or no
// account yet — every domain command is simply absent, and cobra says
// `unknown command "agents"`, which reads like the command does not exist
// rather than like this binary could not ask. cause is what AttachDaemon
// reported.
func ExplainMissingTree(err, cause error) error {
	if err == nil || cause == nil {
		return err
	}
	if !strings.Contains(err.Error(), "unknown command") {
		return err
	}
	return apperr.New("CLI_SURFACE_UNAVAILABLE").
		Causer("clix.AttachDaemon").
		Msgf("this command is published by the daemon, and the daemon could not be reached").
		Issue("cause", cause.Error()).
		Status(apperr.StatusServiceUnavailable).
		Wrap(err).
		CTA(
			apperr.CallToAction{
				Label:   "start the daemon, then run this again",
				Command: build.Name + " gateway start",
				Tool:    "gateway_start",
			},
			apperr.CallToAction{
				Label: "if it is running, this installation has no account yet — create one in the application, which also writes the credential this terminal reads",
			},
		)
}

func errNoPublishedSurface() error {
	return apperr.New("CLI_EMPTY_SURFACE").
		Causer("clix.AttachDaemon").
		Msgf("the daemon answered, but published no commands").
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{
			Label: "the window and the terminal are usually different versions when this happens",
		})
}

// buildDaemonCommand wraps one manifest entry. Unlike BuildCommand, it cannot
// bind typed flags from a Go struct — a daemon across a process boundary
// publishes a key and a summary, not a reflect.Type — so input is generic:
// --json for a full body, repeatable --set path=value for individual fields,
// laid on top of it.
func buildDaemonCommand(info wailsvc.CommandInfo, client wailsvc.Caller, cfg Config) *cobra.Command {
	var jsonBody string
	var sets []string
	var reason string

	c := &cobra.Command{
		Use:   info.Name,
		Short: info.Summary,
		Long: info.Summary + "\n\nRuns against the daemon. Provide input with --json, or with " +
			"repeated --set path=value flags (JSON-decoded when the value parses, a plain " +
			"string otherwise). --reason sets the _reasoning field every command requires, " +
			"and --schema asks the daemon what this command takes instead of running it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := resolveOutput(cmd, cfg)
			applyTransportFlags(cmd, client)

			payload, err := buildPayload(jsonBody, sets, reason)
			if err != nil {
				return err
			}
			// The contract, not the call. This used to be refused outright —
			// "--schema is not available for commands served by the daemon" —
			// which is every command but the four this binary links in, so
			// there was no way at all to discover what a command takes short
			// of sending a wrong payload and reading the refusal (defect #2).
			// The daemon answers it now, on the command's own route.
			if schemaOnly(cmd) {
				payload, err = askForSchema(payload)
				if err != nil {
					return err
				}
			}
			raw, err := client.Invoke(cmd.Context(), info.Key, payload)
			if err != nil {
				return writeError(cfg.Err, err, out)
			}

			var probe struct {
				Error *apperr.Error `json:"error"`
			}
			if uerr := json.Unmarshal(raw, &probe); uerr == nil && probe.Error != nil {
				return writeError(cfg.Err, probe.Error, out)
			}
			var envelope command.Envelope
			if uerr := json.Unmarshal(raw, &envelope); uerr != nil {
				return uerr
			}
			if tokenCountOnly(cmd) {
				return writeTokenCount(cfg.Out, envelope, out)
			}
			return writeResult(cfg.Out, envelope, out)
		},
	}
	c.Flags().StringVar(&jsonBody, flagJSON, "", "full JSON input body")
	c.Flags().StringArrayVar(&sets, flagSet, nil, "set a field of the input: path=value (repeatable)")
	c.Flags().StringVar(&reason, flagReason, "", "why this is being run, sent as _reasoning")
	// The daemon is a remote API even when it is on loopback, so a command it
	// serves gets the flags that say which daemon, as whom, and in which
	// workspace. Their absence here is defect #4: `agents me` resolved an
	// identity and printed it, but nothing could attach one to a call, so
	// every write refused with "this call has no agent identity" and the
	// documented workaround was curl with the header set by hand.
	addTransportFlags(c.Flags())
	return c
}

// askForSchema turns a payload into the question about the payload.
//
// It is spliced into whatever the caller already typed rather than replacing
// it, so `--set category=x --schema` still reads as "what does this take?"
// without the caller having to clear the flags first.
func askForSchema(payload json.RawMessage) (json.RawMessage, error) {
	fields := map[string]any{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &fields); err != nil {
			return nil, err
		}
	}
	fields[command.SchemaField] = true
	return json.Marshal(fields)
}

// identity is the part of a daemon client the transport flags address: which
// daemon, with which credential, in which workspace, as whom.
//
// Each is its own interface so that a client implementing some of them is
// still configurable in those respects — and so that a test double does not
// have to implement four methods to exercise one.
type (
	baseURLSetter   interface{ SetBaseURL(string) }
	tokenSetter     interface{ SetToken(string) }
	workspaceSetter interface{ SetWorkspace(string) }
	agentSetter     interface{ SetAgent(string) }
)

// applyTransportFlags points the client at what this one execution named.
//
// Only flags the caller actually set are applied: an unset --agent must leave
// AOS_AGENT_ID in place rather than clearing the identity the environment
// already established.
func applyTransportFlags(cmd *cobra.Command, client wailsvc.Caller) {
	set := func(name string, apply func(string)) {
		if apply == nil || !cmd.Flags().Changed(name) {
			return
		}
		value, err := cmd.Flags().GetString(name)
		if err != nil {
			return
		}
		apply(strings.TrimSpace(value))
	}
	if c, ok := client.(baseURLSetter); ok {
		set(flagBaseURL, c.SetBaseURL)
	}
	if c, ok := client.(tokenSetter); ok {
		set(flagToken, c.SetToken)
	}
	if c, ok := client.(workspaceSetter); ok {
		set(flagWorkspace, c.SetWorkspace)
	}
	if c, ok := client.(agentSetter); ok {
		set(flagAgent, c.SetAgent)
	}
}

// buildPayload merges --json, --set and --reason into one input body, in that
// order — --set overrides a field --json also set, and --reason always wins,
// since it is the one field every surface but a human's own terminal requires
// and the terminal supplies it explicitly here instead.
func buildPayload(jsonBody string, sets []string, reason string) (json.RawMessage, error) {
	data := map[string]any{}
	if strings.TrimSpace(jsonBody) != "" {
		if err := json.Unmarshal([]byte(jsonBody), &data); err != nil {
			return nil, fmt.Errorf("--json: %w", err)
		}
	}
	for _, kv := range sets {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("--set %q: expected path=value", kv)
		}
		setPath(data, k, parseSetValue(v))
	}
	if reason != "" {
		data["_reasoning"] = reason
	}
	return json.Marshal(data)
}

// setPath assigns value at a dot-separated path within m, creating
// intermediate objects as needed.
func setPath(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = value
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

// parseSetValue reads a --set value as JSON when it parses as one (so
// --set count=3 or --set active=true carry their real type), and as a plain
// string otherwise (so --set name=ada does not have to be quoted).
func parseSetValue(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

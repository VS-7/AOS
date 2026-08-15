// Package clix derives the terminal surface from the command registry.
package clix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/clix/format"
)

// Invoker executes a command. Locally it is the registry itself; from phase 4
// it can also be an HTTP client pointed at a remote daemon by --base-url.
type Invoker interface {
	Invoke(ctx context.Context, d command.Descriptor, raw json.RawMessage) (any, error)
}

// LocalInvoker runs the command in this process.
type LocalInvoker struct{}

func (LocalInvoker) Invoke(ctx context.Context, d command.Descriptor, raw json.RawMessage) (any, error) {
	return d.Invoke(ctx, command.SurfaceCLI, raw)
}

// Config wires the root command.
type Config struct {
	Registry *command.Registry
	Invoker  Invoker
	Out      io.Writer
	Err      io.Writer

	// IsTTY reports whether a human is watching. Nil uses the real terminal.
	IsTTY func() bool
}

func (c *Config) fill() {
	if c.Invoker == nil {
		c.Invoker = LocalInvoker{}
	}
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	if c.IsTTY == nil {
		c.IsTTY = defaultIsTTY
	}
}

// defaultIsTTY reports whether the consumer is a human rather than a program.
// The same signal the original passes as `agent: true`.
func defaultIsTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// NewRoot builds the whole command tree from the registry.
func NewRoot(cfg Config) *cobra.Command {
	cfg.fill()

	root := &cobra.Command{
		Use:           build.Name,
		Short:         build.DisplayName + " — an operating system for AI agents",
		Version:       build.Current().String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(cfg.Out)
	root.SetErr(cfg.Err)
	addGlobalFlags(root.PersistentFlags())

	for _, group := range cfg.Registry.Groups() {
		g := &cobra.Command{
			Use:   group.Name,
			Short: group.Summary,
			Long:  group.Doc,
		}
		for _, d := range group.Commands {
			g.AddCommand(BuildCommand(d, cfg))
		}
		root.AddCommand(g)
	}

	// Built-ins live under a namespace so that they cannot shadow a domain
	// group. In the original, the builtin `skills` group hides the domain one,
	// and `fractal skills --help` shows only `add` and `list` — the domain
	// operations are unreachable from the CLI (defect #15).
	root.AddCommand(selfCommand(cfg))
	return root
}

// BuildCommand turns a descriptor into a cobra command.
func BuildCommand(d command.Descriptor, cfg Config) *cobra.Command {
	b := newBinder(d.InputType())

	use := d.Name()
	for _, a := range b.ArgNames() {
		use += " <" + a + ">"
	}

	c := &cobra.Command{
		Use:     use,
		Aliases: d.Aliases(),
		Short:   d.Summary(),
		Long:    d.Doc(),
		Example: renderExamples(d),
		Args:    cobra.MaximumNArgs(len(b.ArgNames())),
	}
	b.Bind(c.Flags())
	if !d.Local() {
		// A local command never talks to a remote API, so it must not offer
		// flags that suggest it could.
		addTransportFlags(c.Flags())
	}

	c.RunE = func(cmd *cobra.Command, args []string) error {
		out := resolveOutput(cmd, cfg)

		if schemaOnly(cmd) {
			return writeSchema(cfg.Out, d, out)
		}

		raw, err := b.Collect(cmd.Flags(), args)
		if err != nil {
			return err
		}
		result, err := cfg.Invoker.Invoke(cmd.Context(), d, raw)
		if err != nil {
			return writeError(cfg.Err, err, out)
		}
		return writeResult(cfg.Out, command.Wrap(result, nil), out)
	}
	return c
}

func renderExamples(d command.Descriptor) string {
	if len(d.Examples()) == 0 {
		return ""
	}
	var b strings.Builder
	for _, ex := range d.Examples() {
		raw, err := json.Marshal(ex.Input)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "  # %s\n  %s %s --json '%s'\n", ex.Description, build.Name,
			strings.Join(d.Path(), " "), raw)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeSchema(w io.Writer, d command.Descriptor, out format.Options) error {
	raw, err := json.MarshalIndent(d.InputSchema(), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(raw))
	return err
}

func writeResult(w io.Writer, envelope command.Envelope, opts format.Options) error {
	rendered, err := format.Render(envelope, opts)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, rendered.Text)
	return err
}

// writeError renders a failure in the chosen format and returns it, so the
// process exits non-zero. A human sees a message; an agent gets the full JSON,
// including the call to action it should follow.
func writeError(w io.Writer, err error, opts format.Options) error {
	e, ok := apperr.As(err)
	if !ok {
		return err
	}
	rendered, rerr := format.Render(e, opts)
	if rerr != nil {
		return err
	}
	fmt.Fprintln(w, rendered.Text)
	return errSilent{err}
}

// errSilent carries the exit status without printing twice.
type errSilent struct{ error }

func (e errSilent) Unwrap() error { return e.error }

// Silent reports whether an error was already rendered.
func Silent(err error) bool {
	_, ok := err.(errSilent)
	return ok
}

// Package tools holds the native tools: the ones that are not domain commands.
//
// Their names are Claude Code's, deliberately (ADR-0016). A model that has seen
// Read, Write, Edit, Glob, Grep and Bash knows how to use them without being
// taught, and an agent definition written for that tool works here.
//
// Every constructor takes only the sandbox interface its tool needs. Read
// cannot write — not because something checks, but because the value it holds
// has no method that writes.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/runtime/sandbox"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// define builds a tool from its schema and its function, inferring the input
// schema the same way the Command Layer does so that a native tool and a domain
// command describe themselves identically to the model.
func define[In any](name, description string, ann command.Annotations,
	fn func(ctx context.Context, in In) (any, error),
) toolexec.Tool {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		// A schema that cannot be inferred is a programming error in this
		// package, discovered the first time the process starts.
		panic(fmt.Sprintf("tools: %s has an input type that cannot be described: %v", name, err))
	}
	ann.Title = name
	return toolexec.Func{
		Definition: toolexec.Spec{
			Name: name, Description: description,
			InputSchema: schema, Annotations: ann,
		},
		Fn: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in In
			if len(raw) > 0 && string(raw) != "null" {
				if err := json.Unmarshal(raw, &in); err != nil {
					return nil, errBadInput(name, err)
				}
			}
			return fn(ctx, in)
		},
	}
}

// ReadInput is the payload of Read.
type ReadInput struct {
	FilePath string `json:"file_path" jsonschema:"Path to the file, relative to the workspace root."`
	Offset   int    `json:"offset,omitempty" jsonschema:"First line to return, 1-based. Use with limit to read a slice of a large file."`
	Limit    int    `json:"limit,omitempty" jsonschema:"How many lines to return. Defaults to 2000."`

	command.Reasoning
}

// DefaultReadLimit bounds a read that did not ask for a slice. A file with
// eighty thousand lines is not a thing to put in a context window by accident.
const DefaultReadLimit = 2000

// NewRead reads a file. It holds a FileReader and nothing else.
func NewRead(fr sandbox.FileReader) toolexec.Tool {
	return define("Read", `Read a file from the workspace.

Returns the content with 1-based line numbers, which is what makes the offset
and limit parameters useful: read the slice you need rather than the whole file.
This is also how you read a tool output that was spilled to disk.`,
		command.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		func(ctx context.Context, in ReadInput) (any, error) {
			data, err := fr.ReadFile(ctx, in.FilePath)
			if err != nil {
				return nil, err
			}
			lines := strings.Split(string(data), "\n")
			limit := in.Limit
			if limit <= 0 {
				limit = DefaultReadLimit
			}
			start := in.Offset - 1
			if start < 0 {
				start = 0
			}
			if start > len(lines) {
				start = len(lines)
			}
			end := min(start+limit, len(lines))

			var b strings.Builder
			for i := start; i < end; i++ {
				fmt.Fprintf(&b, "%6d\t%s\n", i+1, lines[i])
			}
			return map[string]any{
				"path":      in.FilePath,
				"content":   b.String(),
				"lines":     len(lines),
				"from":      start + 1,
				"to":        end,
				"truncated": end < len(lines),
			}, nil
		})
}

// WriteInput is the payload of Write.
type WriteInput struct {
	FilePath string `json:"file_path" jsonschema:"Path to write, relative to the workspace root. Parent directories are created."`
	Content  string `json:"content" jsonschema:"The complete new content of the file."`

	command.Reasoning
}

// NewWrite writes a file. It holds a FileWriter and cannot read one back.
func NewWrite(fw sandbox.FileWriter) toolexec.Tool {
	return define("Write", `Write a file in the workspace, replacing it if it exists.

Writing replaces the whole file. To change part of one, prefer Edit: a Write
that reconstructs a file from memory is how content the model never saw gets
deleted.`,
		command.Annotations{DestructiveHint: true},
		func(ctx context.Context, in WriteInput) (any, error) {
			if err := fw.WriteFile(ctx, in.FilePath, []byte(in.Content)); err != nil {
				return nil, err
			}
			return map[string]any{"path": in.FilePath, "bytes": len(in.Content)}, nil
		})
}

// EditInput is the payload of Edit.
type EditInput struct {
	FilePath   string `json:"file_path" jsonschema:"Path to the file to change."`
	OldString  string `json:"old_string" jsonschema:"The exact text to replace. Must appear in the file."`
	NewString  string `json:"new_string" jsonschema:"What to put in its place."`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"Replace every occurrence instead of requiring exactly one."`

	command.Reasoning
}

// NewEdit replaces a string in a file.
//
// It needs both halves of the filesystem, and the reason is worth stating: an
// edit is a read and a write, and pretending otherwise would mean giving a
// read-only agent a way to change files.
func NewEdit(fr sandbox.FileReader, fw sandbox.FileWriter) toolexec.Tool {
	return define("Edit", `Replace exact text in a file.

The old text must appear exactly once, unless replace_all is set. That is a
guard rather than an inconvenience: an edit that matches three places when you
meant one is a change you did not review.`,
		command.Annotations{DestructiveHint: true},
		func(ctx context.Context, in EditInput) (any, error) {
			data, err := fr.ReadFile(ctx, in.FilePath)
			if err != nil {
				return nil, err
			}
			body := string(data)
			count := strings.Count(body, in.OldString)
			switch {
			case in.OldString == "":
				return nil, errEmptyMatch(in.FilePath)
			case count == 0:
				return nil, errNoMatch(in.FilePath, in.OldString)
			case count > 1 && !in.ReplaceAll:
				return nil, errAmbiguousMatch(in.FilePath, in.OldString, count)
			}

			var replaced string
			if in.ReplaceAll {
				replaced = strings.ReplaceAll(body, in.OldString, in.NewString)
			} else {
				replaced = strings.Replace(body, in.OldString, in.NewString, 1)
			}
			if err := fw.WriteFile(ctx, in.FilePath, []byte(replaced)); err != nil {
				return nil, err
			}
			return map[string]any{"path": in.FilePath, "replacements": count}, nil
		})
}

// GlobInput is the payload of Glob.
type GlobInput struct {
	Pattern string `json:"pattern" jsonschema:"Glob pattern, for example \"**/*.go\"."`
	Dir     string `json:"path,omitempty" jsonschema:"Directory to search in, relative to the workspace root."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of paths to return. Defaults to 500."`

	command.Reasoning
}

// NewGlob finds files by name.
func NewGlob(g sandbox.Globber) toolexec.Tool {
	return define("Glob", `Find files by pattern.

The .git directory is never searched. Results are paths relative to the
workspace root, sorted, and capped — a search that returns everything is a
search that told you nothing.`,
		command.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		func(ctx context.Context, in GlobInput) (any, error) {
			paths, err := g.Glob(ctx, in.Pattern, sandbox.GlobOptions{Dir: in.Dir, Limit: in.Limit})
			if err != nil {
				return nil, err
			}
			return map[string]any{"paths": paths, "total": len(paths)}, nil
		})
}

// GrepInput is the payload of Grep.
type GrepInput struct {
	Pattern       string `json:"pattern" jsonschema:"Regular expression to search for."`
	Glob          string `json:"glob,omitempty" jsonschema:"Only search files matching this glob. Example: \"**/*.go\"."`
	Dir           string `json:"path,omitempty" jsonschema:"Directory to search in, relative to the workspace root."`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema:"Match case. Off by default."`
	Limit         int    `json:"limit,omitempty" jsonschema:"Maximum number of matching lines to return. Defaults to 200."`

	command.Reasoning
}

// DefaultGrepLimit bounds a search across a repository.
const DefaultGrepLimit = 200

// NewGrep searches file contents.
//
// It is implemented over the sandbox rather than by shelling out to a search
// binary, so it works in an agent whose policy allows no execution at all —
// which is the default policy.
func NewGrep(fr sandbox.FileReader, g sandbox.Globber) toolexec.Tool {
	return define("Grep", `Search the contents of files with a regular expression.

Returns matching lines with their paths and line numbers. Narrow with glob
before widening the pattern: a search of a whole repository for a common word
spends your context on noise.`,
		command.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		func(ctx context.Context, in GrepInput) (any, error) {
			expr := in.Pattern
			if !in.CaseSensitive {
				expr = "(?i)" + expr
			}
			re, err := regexp.Compile(expr)
			if err != nil {
				return nil, errBadPattern(in.Pattern, err)
			}
			pattern := in.Glob
			if pattern == "" {
				pattern = "**"
			}
			paths, err := g.Glob(ctx, pattern, sandbox.GlobOptions{Dir: in.Dir, Limit: 0})
			if err != nil {
				return nil, err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = DefaultGrepLimit
			}

			type match struct {
				Path string `json:"path"`
				Line int    `json:"line"`
				Text string `json:"text"`
			}
			var matches []match
			var scanned int
			for _, p := range paths {
				if len(matches) >= limit {
					break
				}
				data, err := fr.ReadFile(ctx, p)
				if err != nil {
					continue // a file that cannot be read is not a match
				}
				scanned++
				for i, line := range strings.Split(string(data), "\n") {
					if !re.MatchString(line) {
						continue
					}
					matches = append(matches, match{Path: p, Line: i + 1, Text: strings.TrimRight(line, "\r")})
					if len(matches) >= limit {
						break
					}
				}
			}
			return map[string]any{
				"matches": matches, "total": len(matches),
				"filesScanned": scanned, "truncated": len(matches) >= limit,
			}, nil
		})
}

// BashInput is the payload of Bash.
type BashInput struct {
	Command   string   `json:"command" jsonschema:"The program to run. This is the binary name, not a shell line."`
	Args      []string `json:"args,omitempty" jsonschema:"Arguments, one per element. Do not put a whole command line in a single string."`
	Dir       string   `json:"path,omitempty" jsonschema:"Working directory, relative to the workspace root."`
	TimeoutMS int      `json:"timeout_ms,omitempty" jsonschema:"How long to allow, in milliseconds. Defaults to 120000."`

	command.Reasoning
}

// NewBash runs a program.
//
// It takes a binary and a list of arguments rather than a command line, which
// is the shape ADR-0006 needs: a string that gets handed to a shell has no
// binary to resolve, and resolving the binary is the whole of the allowlist.
// An agent that genuinely needs a pipeline names its shell, and the policy has
// to allow that explicitly.
func NewBash(cr sandbox.CommandRunner) toolexec.Tool {
	return define("Bash", `Run a program from this agent's allowlist.

Give the binary and its arguments separately. This does not go through a shell:
there is no pipe, no redirection and no globbing unless the program does it. If
the binary is not on the allowlist the error names the exact line to add to the
agent's policy — that is a request to make, not an obstacle to route around.

The result carries how much of the output was omitted. A truncated build log
that looks like it passed is the failure mode this exists to prevent.`,
		command.Annotations{DestructiveHint: true, OpenWorldHint: true},
		func(ctx context.Context, in BashInput) (any, error) {
			var timeout time.Duration
			if in.TimeoutMS > 0 {
				timeout = time.Duration(in.TimeoutMS) * time.Millisecond
			}
			res, err := cr.Run(ctx, sandbox.Command{
				Name: in.Command, Args: in.Args, Dir: in.Dir, Timeout: timeout,
			})
			if err != nil {
				return nil, err
			}
			return res, nil
		})
}

// FS builds the filesystem toolset over one sandbox.
//
// The sandbox is passed once and each tool takes the slice of it that it needs,
// which is the interface segregation the specification asks for and the reason
// a read-only agent's Read tool has no path to a write.
func FS(s *sandbox.Sandbox) []toolexec.Tool {
	return []toolexec.Tool{
		NewRead(s), NewWrite(s), NewEdit(s, s), NewGlob(s), NewGrep(s, s), NewBash(s),
	}
}

func errBadInput(tool string, cause error) error {
	return apperr.New("TOOL_INPUT_INVALID").
		Causer("tools."+tool).
		Msgf("the payload for %s could not be read", tool).
		Issue("tool", tool).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "read the tool's input schema, then send only the fields it lists"})
}

func errBadPattern(pattern string, cause error) error {
	return apperr.New("TOOL_PATTERN_INVALID").
		Causer("tools.Grep").
		Msgf("%q is not a valid regular expression", pattern).
		Issue("pattern", pattern).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "escape the metacharacters you meant literally, or search for a plainer substring"})
}

func errEmptyMatch(path string) error {
	return apperr.New("TOOL_EDIT_EMPTY_MATCH").
		Causer("tools.Edit").
		Msgf("an edit needs text to replace").
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "to create a file or replace it whole, use Write"})
}

func errNoMatch(path, old string) error {
	return apperr.New("TOOL_EDIT_NO_MATCH").
		Causer("tools.Edit").
		Msgf("the text to replace does not appear in %s", path).
		Issue("path", path).
		Issue("length", len(old)).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "read the file again and copy the text exactly, including its indentation",
			Tool:  "Read",
		})
}

func errAmbiguousMatch(path, old string, count int) error {
	return apperr.New("TOOL_EDIT_AMBIGUOUS").
		Causer("tools.Edit").
		Msgf("the text to replace appears %d times in %s", count, path).
		Issue("path", path).
		Issue("occurrences", count).
		Issue("length", len(old)).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "include enough surrounding lines to make the match unique, or set replace_all if you meant all of them",
		})
}

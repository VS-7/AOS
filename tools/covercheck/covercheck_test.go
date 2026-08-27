package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// runCovercheck builds and runs the command the way `task cover` does — over
// stdin — and returns its exit code and everything it printed. Building it
// rather than calling main() is what lets the exit code be part of the
// assertion: the gate's answer *is* its exit code.
func runCovercheck(t *testing.T, stdin string) (int, string) {
	t.Helper()
	binary := t.TempDir() + "/covercheck"
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building covercheck: %v\n%s", err, out)
	}

	cmd := exec.CommandContext(t.Context(), binary)
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running covercheck: %v\n%s", err, buf.String())
		}
		return exit.ExitCode(), buf.String()
	}
	return 0, buf.String()
}

// A run that ended early is not a run that passed.
//
// `go test -cover ./...` can stop halfway — a package that fails to build, a
// full disk, an interrupt — and every line covercheck never saw is a floor it
// never enforced. It used to print "1 packages, all at or above their floor"
// and exit 0, which is a gate answering a question nobody asked. Observed:
// a disk that filled mid-run turned a hundred-and-twenty-five-package check
// into a one-package check, and the gate said yes.
func TestATruncatedRunIsRefused(t *testing.T) {
	one := "ok  \tgithub.com/OWNER/aos/internal/core/pathx\t0.1s\tcoverage: 95.0% of statements\n"

	code, out := runCovercheck(t, one)
	if code == 0 {
		t.Fatalf("a run reporting one package passed:\n%s", out)
	}
	if !strings.Contains(out, "did not finish") {
		t.Errorf("the reason was not stated:\n%s", out)
	}
}

// A complete run still passes, and one below a floor still fails — the two
// answers the gate exists to give.
func TestACompleteRunIsJudgedOnTheFloors(t *testing.T) {
	var clean strings.Builder
	for i := 0; i < minimumPackages; i++ {
		fmt.Fprintf(&clean, "ok  \tgithub.com/OWNER/aos/internal/adapters/p%d\t0.1s\tcoverage: 0.0%% of statements\n", i)
	}
	if code, out := runCovercheck(t, clean.String()); code != 0 {
		t.Fatalf("a complete run of packages at their floor was refused:\n%s", out)
	}

	below := clean.String() +
		"ok  \tgithub.com/OWNER/aos/internal/core/pathx\t0.1s\tcoverage: 12.0% of statements\n"
	code, out := runCovercheck(t, below)
	if code == 0 {
		t.Fatalf("a package under its floor passed:\n%s", out)
	}
	if !strings.Contains(out, "internal/core/pathx") {
		t.Errorf("the failing package was not named:\n%s", out)
	}
}

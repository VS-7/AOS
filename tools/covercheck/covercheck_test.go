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

// A test that fails has to be visible, because this command owns the stdout
// of the pipeline that runs the tests.
//
// `go test -cover ./... | covercheck` sends every line here, and every line
// without "coverage:" used to be dropped — so a package whose tests failed
// vanished, covercheck printed "all at or above their floor", and the step
// exited 1 on `pipefail` with nothing in the log saying what broke. Observed
// on CI: a flaky test failed inside the coverage step and the only evidence
// was the package count being one lower than usual.
func TestAFailingPackageIsNamedRatherThanSwallowed(t *testing.T) {
	var run strings.Builder
	for i := 0; i < minimumPackages; i++ {
		fmt.Fprintf(&run, "ok  \tgithub.com/OWNER/aos/internal/adapters/p%d\t0.1s\tcoverage: 0.0%% of statements\n", i)
	}
	run.WriteString("--- FAIL: TestSomethingRacy (0.01s)\n")
	run.WriteString("    thing_test.go:12: subscribers = 3\n")
	run.WriteString("FAIL\tgithub.com/OWNER/aos/internal/transport/realtime\t0.2s\n")

	code, out := runCovercheck(t, run.String())
	if code == 0 {
		t.Fatalf("a run with a failing package passed:\n%s", out)
	}
	if !strings.Contains(out, "internal/transport/realtime") {
		t.Errorf("the failing package was not named:\n%s", out)
	}
	// The line carrying the message is what a person actually needs.
	if !strings.Contains(out, "TestSomethingRacy") {
		t.Errorf("the failing test was not shown:\n%s", out)
	}
	if strings.Contains(out, "all at or above their floor") {
		t.Errorf("a run with a failing package was reported as clean:\n%s", out)
	}
}

// A package that does not compile: the header, the compiler's own message,
// and the verdict that names it — all three of which `go test` really prints,
// checked against it rather than imagined.
func TestAPackageThatDoesNotBuildIsVisible(t *testing.T) {
	var run strings.Builder
	for i := 0; i < minimumPackages; i++ {
		fmt.Fprintf(&run, "ok  \tgithub.com/OWNER/aos/internal/adapters/p%d\t0.1s\tcoverage: 0.0%% of statements\n", i)
	}
	run.WriteString("# github.com/OWNER/aos/internal/domain/goal [github.com/OWNER/aos/internal/domain/goal.test]\n")
	run.WriteString("internal/domain/goal/goal.go:12:2: undefined: thing\n")
	run.WriteString("FAIL\tgithub.com/OWNER/aos/internal/domain/goal [build failed]\n")

	code, out := runCovercheck(t, run.String())
	if code == 0 {
		t.Fatalf("a run with a package that does not build passed:\n%s", out)
	}
	if !strings.Contains(out, "undefined: thing") {
		t.Errorf("the compiler's message was dropped:\n%s", out)
	}
	if !strings.Contains(out, "internal/domain/goal") {
		t.Errorf("the failing package was not named:\n%s", out)
	}
}

// The "# pkg" header is not a failure. Every cgo build on macOS prints one,
// with linker warnings under it, for a package whose tests then pass — and
// reading it as a failure marker turned a clean local run into a red gate.
func TestALinkerWarningIsNotAFailure(t *testing.T) {
	var run strings.Builder
	for i := 0; i < minimumPackages; i++ {
		fmt.Fprintf(&run, "ok  \tgithub.com/OWNER/aos/internal/adapters/p%d\t0.1s\tcoverage: 0.0%% of statements\n", i)
	}
	run.WriteString("# github.com/OWNER/aos/cmd/aos-desktop.test\n")
	run.WriteString("ld: warning: object file was built for newer macOS version than being linked\n")
	run.WriteString("ok  \tgithub.com/OWNER/aos/cmd/aos-desktop\t2.0s\tcoverage: 0.0% of statements\n")

	if code, out := runCovercheck(t, run.String()); code != 0 {
		t.Fatalf("a warning was read as a failure:\n%s", out)
	}
}

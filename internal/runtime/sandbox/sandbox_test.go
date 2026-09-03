package sandbox_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/runtime/sandbox"
)

func full() sandbox.Permissions {
	return sandbox.Permissions{Read: true, Write: true, Delete: true, Execute: true}
}

// build makes a sandbox over a temporary workspace with a .git directory, a
// file to read, and a spillover directory outside the root.
func build(t *testing.T, o sandbox.Options) (*sandbox.Sandbox, string, string) {
	t.Helper()
	root := t.TempDir()
	tmp := t.TempDir()

	mustWrite(t, filepath.Join(root, "README.md"), "hello")
	mustWrite(t, filepath.Join(root, "src", "main.go"), "package main")
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")
	mustWrite(t, filepath.Join(root, ".git", "objects", "ab", "cdef"), "object")
	mustWrite(t, filepath.Join(tmp, "spilled.txt"), "the rest of the output")

	o.WorkspacePath = root
	o.TmpDir = tmp
	if (o.Permissions == sandbox.Permissions{}) {
		o.Permissions = full()
	}
	s, err := sandbox.New(o)
	if err != nil {
		t.Fatal(err)
	}
	// The root is resolved through symlinks, which on macOS turns /var into
	// /private/var. Tests compare against what the sandbox decided.
	return s, s.Root(), tmp
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}

// TestAPathThatLeavesTheRootIsRefused. Normalising happens before containment
// is checked, so every one of these is simply a path that is not in the
// workspace by the time anything looks at it.
func TestAPathThatLeavesTheRootIsRefused(t *testing.T) {
	s, root, _ := build(t, sandbox.Options{})
	ctx := context.Background()

	cases := []struct {
		name, path string
	}{
		{"the classic traversal", "../../etc/passwd"},
		{"a traversal dressed up", "./src/../../../etc/passwd"},
		{"an absolute path elsewhere", "/etc/passwd"},
		{"the parent of the root", filepath.Dir(root)},
		{"a sibling whose name starts with the root", root + "-sibling/file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.ReadFile(ctx, c.path); err == nil {
				t.Fatal("the read was allowed")
			} else if got := codeOf(t, err); got != "AOS_SANDBOX_PATH_OUTSIDE" {
				t.Fatalf("code = %s: %v", got, err)
			}
		})
	}
}

// TestARootIsNotAPrefix is the classic bug: comparing "/a/b" against "/a/bc"
// with strings.HasPrefix says yes, and it is the wrong answer.
func TestARootIsNotAPrefix(t *testing.T) {
	parent := t.TempDir()
	inside := filepath.Join(parent, "b")
	sibling := filepath.Join(parent, "bc")
	mustWrite(t, filepath.Join(inside, "ok.txt"), "in")
	mustWrite(t, filepath.Join(sibling, "no.txt"), "out")

	s, err := sandbox.New(sandbox.Options{WorkspacePath: inside, Permissions: full()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadFile(context.Background(), filepath.Join(sibling, "no.txt")); err == nil {
		t.Fatal("/a/bc was treated as being inside /a/b")
	}
}

// TestASymlinkIsNotAnEscapeHatch. This is the addition: the original resolves
// "..", and a link inside the root pointing outside it would otherwise be a way
// out that nobody sees, because the path that goes in looks local.
func TestASymlinkIsNotAnEscapeHatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need a privilege on Windows that a test should not assume")
	}
	s, root, _ := build(t, sandbox.Options{})
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"), "not for the agent")

	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := s.ReadFile(ctx, "escape/secret.txt"); err == nil {
		t.Fatal("a symlink out of the root was followed")
	} else if got := codeOf(t, err); got != "AOS_SANDBOX_PATH_OUTSIDE" {
		t.Fatalf("code = %s", got)
	}

	// The write path too: a new file under a symlinked directory resolves its
	// parent, which is the only reason this is caught.
	if err := s.WriteFile(ctx, "escape/planted.txt", []byte("x")); err == nil {
		t.Fatal("a write through a symlink left the root")
	}
}

// TestTheGitDirectoryIsReadableAndNotWritable, at the top level and below it.
func TestTheGitDirectoryIsReadableAndNotWritable(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{})
	ctx := context.Background()

	if _, err := s.ReadFile(ctx, ".git/HEAD"); err != nil {
		t.Fatalf("the agent cannot read git history: %v", err)
	}
	for _, path := range []string{".git/HEAD", ".git/objects/ab/cdef", ".git/hooks/pre-commit"} {
		if err := s.WriteFile(ctx, path, []byte("x")); err == nil {
			t.Errorf("%s was written", path)
		} else if got := codeOf(t, err); got != "AOS_SANDBOX_GIT_READ_ONLY" {
			t.Errorf("%s: code = %s", path, got)
		}
		if err := s.Remove(ctx, path); err == nil {
			t.Errorf("%s was removed", path)
		}
	}
}

// TestTheSpilloverDirectoryIsReadableAndNotWritable. The agent goes back for
// the part of a large output that did not fit, and cannot use the directory as
// a scratch space outside the workspace.
func TestTheSpilloverDirectoryIsReadableAndNotWritable(t *testing.T) {
	s, _, tmp := build(t, sandbox.Options{})
	ctx := context.Background()

	got, err := s.ReadFile(ctx, filepath.Join(tmp, "spilled.txt"))
	if err != nil {
		t.Fatalf("the agent cannot read its own spilled output: %v", err)
	}
	if string(got) != "the rest of the output" {
		t.Fatalf("read %q", got)
	}
	if err := s.WriteFile(ctx, filepath.Join(tmp, "planted.txt"), []byte("x")); err == nil {
		t.Fatal("the spillover directory was written to")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_TMP_READ_ONLY" {
		t.Fatalf("code = %s", code)
	}
}

// TestAnAgentWithNoPolicyCanOnlyRead. The default is stricter than the
// original's, which defaults to read and then hands over unrestricted
// execution as soon as the execute permission appears.
func TestAnAgentWithNoPolicyCanOnlyRead(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{Permissions: sandbox.DefaultPermissions()})
	ctx := context.Background()

	if _, err := s.ReadFile(ctx, "README.md"); err != nil {
		t.Fatalf("the default cannot read: %v", err)
	}
	if err := s.WriteFile(ctx, "new.txt", []byte("x")); err == nil {
		t.Error("the default could write")
	}
	if err := s.Remove(ctx, "README.md"); err == nil {
		t.Error("the default could delete")
	}
	if _, err := s.VerifyExec("ls", nil); err == nil {
		t.Error("the default could execute")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_EXEC_PERMISSION_REQUIRED" {
		t.Errorf("code = %s", code)
	}
}

// TestDeletingTakesItsOwnPermission. Overwriting a file is one kind of mistake
// and removing a tree is another.
func TestDeletingTakesItsOwnPermission(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{
		Permissions: sandbox.Permissions{Read: true, Write: true},
	})
	ctx := context.Background()
	if err := s.WriteFile(ctx, "notes.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, "notes.md"); err == nil {
		t.Fatal("write implied delete")
	}
}

// TestTheRootItselfCannotBeRemoved.
func TestTheRootItselfCannotBeRemoved(t *testing.T) {
	s, root, _ := build(t, sandbox.Options{})
	if err := s.Remove(context.Background(), root); err == nil {
		t.Fatal("the workspace root was removed")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_ROOT_REMOVAL" {
		t.Fatalf("code = %s", code)
	}
}

// TestTheDocumentedBypassesOfTheOriginalAreAllRefused.
//
// Each line names the bypass from the reverse engineering that it addresses. A
// blocklist by basename lets every one of them through, which is why ADR-0006
// replaces it: a protection that does not protect is worse than none, because
// the person granting the permission believes they are covered.
func TestTheDocumentedBypassesOfTheOriginalAreAllRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the bypass table is written against POSIX binaries")
	}
	// git is allowed here, and nothing else: the realistic policy of an agent
	// that works on a repository.
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{
			Policy: sandbox.PolicyAllowlist,
			Allow:  []string{"git"},
			DenyArgs: []string{
				"git push --force*",
				"git clean*",
				"* --no-verify*",
			},
		},
	})

	cases := []struct {
		bypass string
		name   string
		args   []string
		want   string
	}{
		{`bash -c "rm -rf /"`, "bash", []string{"-c", "rm -rf /"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`/bin/sh -c 'dd if=...'`, "/bin/sh", []string{"-c", "dd if=/dev/zero of=/dev/sda"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`python -c "shutil.rmtree(...)"`, "python3", []string{"-c", "import shutil; shutil.rmtree('/')"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`find . -delete`, "find", []string{".", "-delete"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`node -e "fs.rmSync(...)"`, "node", []string{"-e", "require('fs').rmSync('/', {recursive:true})"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`rm -rf /`, "rm", []string{"-rf", "/"}, "AOS_SANDBOX_EXEC_NOT_ALLOWED"},
		{`git clean -fdx`, "git", []string{"clean", "-fdx"}, "AOS_SANDBOX_ARGS_DENIED"},
		{`git push --force`, "git", []string{"push", "--force", "origin", "main"}, "AOS_SANDBOX_ARGS_DENIED"},
	}

	for _, c := range cases {
		t.Run(c.bypass, func(t *testing.T) {
			_, err := s.VerifyExec(c.name, c.args)
			if err == nil {
				t.Fatalf("%s was allowed", c.bypass)
			}
			got := codeOf(t, err)
			// A binary that is not installed on this machine reports that
			// instead, which is also a refusal — but it is a weaker one, so
			// the test says which it got.
			if got == "AOS_SANDBOX_EXEC_NOT_FOUND" {
				t.Skipf("%s is not installed here; refused as not found", c.name)
			}
			if got != c.want {
				t.Fatalf("code = %s, want %s", got, c.want)
			}
		})
	}
}

// TestTheSameBinaryUnderTwoNamesIsOneTarget. Resolution is on disk, so the
// spelling the caller chose does not decide the answer.
func TestTheSameBinaryUnderTwoNamesIsOneTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX paths")
	}
	real, err := exec.LookPath("ls")
	if err != nil {
		t.Skip("ls is not installed")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"ls"}},
	})

	for _, spelling := range []string{"ls", real} {
		if _, err := s.VerifyExec(spelling, nil); err != nil {
			t.Errorf("%q was refused: %v", spelling, err)
		}
	}
	// And the same in reverse: an allowlist naming the absolute path accepts
	// the bare name.
	byPath, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{real}},
	})
	if _, err := byPath.VerifyExec("ls", nil); err != nil {
		t.Errorf("the bare name was refused against an absolute allowlist entry: %v", err)
	}
}

// TestADistributionSymlinkDoesNotVoidTheAllowlist: a name the system ships as
// a link to a differently named binary is still the name a person writes on
// the allowlist. /usr/bin/vi points at vim, and on Debian and Ubuntu
// /usr/bin/sh points at dash.
//
// This is the defect that kept the suite red on Linux once CI could finally
// reach it: resolving to the canonical file and then comparing only that name
// made `allow: [sh]` unsatisfiable there, while the same agent frontmatter
// worked on macOS and said nothing about why.
func TestADistributionSymlinkDoesNotVoidTheAllowlist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the controlled PATH and its links are POSIX here")
	}
	name, target := aliasedBinary(t)
	if name == "" {
		t.Skip("this system ships no aliased binary on the controlled PATH")
	}

	// AllowShell because the alias found may well be a shell — on Debian the
	// first one is sh — and this test is about the allowlist, not about the
	// separate opt-in a shell needs.
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{
			Policy: sandbox.PolicyAllowlist, Allow: []string{name}, AllowShell: true,
		},
	})
	resolved, err := s.VerifyExec(name, nil)
	if err != nil {
		t.Fatalf("%q is on the allowlist by that name and was refused: %v", name, err)
	}
	// And what would run is still the file the link points at, not the link:
	// the allowlist got more forgiving, the resolution did not.
	if resolved != target {
		t.Errorf("resolved to %q; the canonical target is %q", resolved, target)
	}
}

// aliasedBinary finds a binary the controlled PATH reaches under a name that
// differs from the name of the file it resolves to, and returns both. Which
// one it is varies by system, and a system that ships none has nothing to
// prove here.
func aliasedBinary(t *testing.T) (string, string) {
	t.Helper()
	// The same directories, in the same order, that the sandbox searches.
	dirs := []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/usr/local/bin"}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Type()&os.ModeSymlink == 0 {
				continue
			}
			link := filepath.Join(dir, e.Name())
			target, err := filepath.EvalSymlinks(link)
			if err != nil || filepath.Base(target) == e.Name() {
				continue
			}
			// Only useful if this is the file the sandbox itself would find
			// for that bare name; an earlier directory may carry the name too.
			if firstOnPath(dirs, e.Name()) != link {
				continue
			}
			return e.Name(), target
		}
	}
	return "", ""
}

func firstOnPath(dirs []string, name string) string {
	for _, dir := range dirs {
		if found, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return found
		}
	}
	return ""
}

// TestAShellNeedsItsOwnPermission, and is still subject to the denied
// patterns once it has it.
func TestAShellNeedsItsOwnPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shells")
	}
	deny := []string{"* rm -rf *"}

	without, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"sh"}, DenyArgs: deny},
	})
	if _, err := without.VerifyExec("sh", []string{"-c", "echo hi"}); err == nil {
		t.Fatal("a shell ran without allowShell")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_SHELL_NOT_ALLOWED" {
		t.Fatalf("code = %s", code)
	}

	with, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{
			Policy: sandbox.PolicyAllowlist, Allow: []string{"sh"},
			DenyArgs: deny, AllowShell: true,
		},
	})
	if _, err := with.VerifyExec("sh", []string{"-c", "echo hi"}); err != nil {
		t.Fatalf("an explicitly allowed shell was refused: %v", err)
	}
	if _, err := with.VerifyExec("sh", []string{"-c", "rm -rf /tmp/x"}); err == nil {
		t.Fatal("the denied pattern did not apply inside the shell's arguments")
	}
}

// TestDenyAllIsTheDefaultPolicy.
func TestDenyAllIsTheDefaultPolicy(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{Permissions: full()})
	if _, err := s.VerifyExec("ls", nil); err == nil {
		t.Fatal("a sandbox with no exec policy ran a command")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_EXEC_NOT_ALLOWED" {
		t.Fatalf("code = %s", code)
	}
}

// TestTheRefusalSaysExactlyWhatToAdd. An allowlist creates friction; an error
// that only says no turns it into a wall.
func TestTheRefusalSaysExactlyWhatToAdd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	if _, err := exec.LookPath("ls"); err != nil {
		t.Skip("ls is not installed")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"git"}},
	})
	_, err := s.VerifyExec("ls", nil)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err = %v", err)
	}
	if len(app.Actions) == 0 || !strings.Contains(app.Actions[0].Command, `"ls"`) {
		t.Fatalf("the error does not carry the line to add: %+v", app.Actions)
	}
	if app.Issues["resolved"] == nil {
		t.Error("the error does not say which binary it resolved")
	}
}

// TestACommandThatOverrunsIsKilled, and says so rather than reporting an empty
// success.
func TestACommandThatOverrunsIsKilled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX sleep")
	}
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep is not installed")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"sleep"}},
	})

	start := time.Now()
	res, err := s.Run(context.Background(), sandbox.Command{
		Name: "sleep", Args: []string{"30"}, Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut {
		t.Fatal("the command overran and the result does not say so")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("the deadline did not bite: %s", elapsed)
	}
}

// TestTheAgentIsToldHowMuchItDidNotSee. Without OmittedSize the agent reads a
// truncated build log and concludes the build passed.
func TestTheAgentIsToldHowMuchItDidNotSee(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skip("printf is not installed as a binary here")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec:           sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"printf"}},
		MaxOutputChars: 64,
	})
	res, err := s.Run(context.Background(), sandbox.Command{
		Name: "printf", Args: []string{strings.Repeat("x", 500)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stdout.Content) > 64 {
		t.Fatalf("the visible output is %d chars", len(res.Stdout.Content))
	}
	if res.Stdout.OriginalSize != 500 {
		t.Errorf("OriginalSize = %d, want 500", res.Stdout.OriginalSize)
	}
	if res.Stdout.OmittedSize != 500-len(res.Stdout.Content) {
		t.Errorf("OmittedSize = %d", res.Stdout.OmittedSize)
	}
}

// TestTruncationDoesNotSplitACharacter. The original guards UTF-16 surrogate
// pairs; in Go the same concern is the rune boundary.
func TestTruncationDoesNotSplitACharacter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skip("printf is not installed as a binary here")
	}
	// Each of these is three bytes, so a limit of 50 lands mid-character.
	s, _, _ := build(t, sandbox.Options{
		Exec:           sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"printf"}},
		MaxOutputChars: 50,
	})
	res, err := s.Run(context.Background(), sandbox.Command{
		Name: "printf", Args: []string{strings.Repeat("日", 40)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !utf8Valid(res.Stdout.Content) {
		t.Fatalf("the truncated output is not valid UTF-8: %q", res.Stdout.Content)
	}
}

// TestTheChildCannotReadTheDaemonSecrets. An addition: the original passes the
// parent's environment through, so `env` in an allowed shell prints the key.
func TestTheChildCannotReadTheDaemonSecrets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX env")
	}
	if _, err := exec.LookPath("env"); err != nil {
		t.Skip("env is not installed")
	}
	t.Setenv("AOS_TOKEN", "secret-token-value")
	t.Setenv("OPENAI_API_KEY", "sk-secret")

	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"env"}},
	})
	res, err := s.Run(context.Background(), sandbox.Command{Name: "env"})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-token-value", "sk-secret"} {
		if strings.Contains(res.Stdout.Content, secret) {
			t.Errorf("the child process can read %s", secret)
		}
	}
	if !strings.Contains(res.Stdout.Content, "PATH=") {
		t.Error("the child has no PATH and cannot work")
	}
}

// TestAFailingCommandIsAResultAndNotAnError. A non-zero exit is information
// the agent reasons about; turning it into a Go error would abort the turn.
func TestAFailingCommandIsAResultAndNotAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false is not installed")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"false"}},
	})
	res, err := s.Run(context.Background(), sandbox.Command{Name: "false"})
	if err != nil {
		t.Fatalf("a non-zero exit was reported as a failure of the call: %v", err)
	}
	if res.ExitCode == 0 {
		t.Fatal("the exit code was lost")
	}
}

// TestGlobNeverReturnsTheGitDirectory, and stays inside the root.
func TestGlobNeverReturnsTheGitDirectory(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{})
	got, err := s.Glob(context.Background(), "**", sandbox.GlobOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("the search found nothing at all")
	}
	for _, p := range got {
		if strings.HasPrefix(p, ".git") {
			t.Errorf("the search returned %s", p)
		}
		if strings.HasPrefix(p, "..") || filepath.IsAbs(p) {
			t.Errorf("the search returned a path outside the root: %s", p)
		}
	}
}

// TestGlobRespectsItsPatternAndItsLimit.
func TestGlobRespectsItsPatternAndItsLimit(t *testing.T) {
	s, root, _ := build(t, sandbox.Options{})
	for i := range 20 {
		mustWrite(t, filepath.Join(root, "many", "f"+string(rune('a'+i))+".txt"), "x")
	}
	ctx := context.Background()

	got, err := s.Glob(ctx, "**/*.go", sandbox.GlobOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "src/main.go" {
		t.Fatalf("pattern match = %v", got)
	}

	limited, err := s.Glob(ctx, "**", sandbox.GlobOptions{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 5 {
		t.Fatalf("the limit was not applied: %d results", len(limited))
	}

	scoped, err := s.Glob(ctx, "*.go", sandbox.GlobOptions{Dir: "src"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 {
		t.Fatalf("a scoped search returned %v", scoped)
	}
}

// TestGlobOutsideTheRootIsRefused.
func TestGlobOutsideTheRootIsRefused(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{})
	if _, err := s.Glob(context.Background(), "**", sandbox.GlobOptions{Dir: "../.."}); err == nil {
		t.Fatal("a search escaped the root")
	}
}

// TestStatReportsWithoutReading.
func TestStatReportsWithoutReading(t *testing.T) {
	s, _, _ := build(t, sandbox.Options{})
	info, err := s.Stat(context.Background(), "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != "README.md" || info.Size != 5 || info.IsDir {
		t.Fatalf("info = %+v", info)
	}
}

// TestATaskWorktreeReplacesTheRoot, which is how a task on its own branch is
// kept away from the main checkout.
func TestATaskWorktreeReplacesTheRoot(t *testing.T) {
	main := t.TempDir()
	worktree := t.TempDir()
	mustWrite(t, filepath.Join(main, "main-only.txt"), "x")
	mustWrite(t, filepath.Join(worktree, "branch.txt"), "y")

	s, err := sandbox.New(sandbox.Options{
		WorkspacePath: main, WorktreePath: worktree, Permissions: full(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadFile(context.Background(), "branch.txt"); err != nil {
		t.Fatalf("the worktree is not readable: %v", err)
	}
	if _, err := s.ReadFile(context.Background(), filepath.Join(main, "main-only.txt")); err == nil {
		t.Fatal("the main checkout is reachable from a task worktree")
	}
}

// TestASandboxNeedsAnAbsoluteExistingRoot.
func TestASandboxNeedsAnAbsoluteExistingRoot(t *testing.T) {
	cases := map[string]sandbox.Options{
		"no root at all":  {},
		"a relative root": {WorkspacePath: "relative/path"},
		"a missing root":  {WorkspacePath: filepath.Join(t.TempDir(), "nope")},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := sandbox.New(o); err == nil {
				t.Fatal("the sandbox was built")
			}
		})
	}
}

// TestPermissionsRoundTripThroughTheFrontMatterForm.
func TestPermissionsRoundTripThroughTheFrontMatterForm(t *testing.T) {
	p := sandbox.PermissionsFrom([]string{"read", " WRITE ", "execute", "nonsense"})
	if !p.Read || !p.Write || !p.Execute || p.Delete {
		t.Fatalf("parsed = %+v", p)
	}
	if got := strings.Join(p.List(), ","); got != "read,write,execute" {
		t.Fatalf("List = %s", got)
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

// TestADenialPatternSpansPathSeparators. A command line is not a path: under
// path-glob semantics `*` stops at a slash, and `* --no-verify*` would quietly
// fail to match any command that mentions a directory — which is most of them.
func TestADenialPatternSpansPathSeparators(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	s, _, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{
			Policy:   sandbox.PolicyAllowlist,
			Allow:    []string{"git"},
			DenyArgs: []string{"* --no-verify*"},
		},
	})

	if _, err := s.VerifyExec("git", []string{"-C", "/srv/repo", "commit", "--no-verify", "-m", "x"}); err == nil {
		t.Fatal("the pattern did not match a line containing a path")
	} else if code := codeOf(t, err); code != "AOS_SANDBOX_ARGS_DENIED" {
		t.Fatalf("code = %s", code)
	}
	if _, err := s.VerifyExec("git", []string{"-C", "/srv/repo", "commit", "-m", "x"}); err != nil {
		t.Fatalf("an ordinary commit was refused: %v", err)
	}
}

// A workspace file named after an allowed binary is not that binary.
//
// The allowlist matches on a basename, and lookPath used to hand it the
// basename of whatever file the caller pointed at — so `allow: [git]` ran
// any executable called `git` the sandbox could reach, the workspace's own
// included. Placing one there needs no privilege: an already-allowed `cp`,
// a `tar`, or a repository that simply ships the file.
func TestAnExecutableInTheWorkspaceCannotImpersonateAnAllowedBinary(t *testing.T) {
	s, root, _ := build(t, sandbox.Options{
		Exec: sandbox.ExecPolicy{Policy: "allowlist", Allow: []string{"git"}},
	})
	for _, rel := range []string{"git", filepath.Join("scripts", "git")} {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("#!/bin/sh\necho impersonated\n"), 0o755); err != nil { //nolint:gosec // deliberately executable: that is the scenario
			t.Fatal(err)
		}
	}

	for _, name := range []string{"./git", "scripts/git", "./scripts/git", filepath.Join(root, "git")} {
		if _, err := s.VerifyExec(name, nil); err == nil {
			t.Errorf("%q was accepted; a file in the workspace must never satisfy an allowlist entry", name)
		}
	}

	// The real binary still runs, by every spelling that names it.
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git on this machine to prove the other half with")
	}
	for _, name := range []string{"git", real} {
		if _, err := s.VerifyExec(name, nil); err != nil {
			t.Errorf("%q was refused, but it is the allowed binary: %v", name, err)
		}
	}
}

// The most common mistake a model makes with Bash is a whole command line in
// `command`. Reported as "not installed" — which is what looking up the whole
// line looks like — it sent the model hunting for a program that was there,
// and the call to action pointed at the allowlist.
func TestACommandLineIsNotReportedAsAMissingProgram(t *testing.T) {
	box, err := sandbox.New(sandbox.Options{
		WorkspacePath: t.TempDir(),
		Permissions:   sandbox.PermissionsFrom([]string{"read", "execute"}),
		Exec:          sandbox.ExecPolicy{Policy: "allowlist", Allow: []string{"ls"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = box.VerifyExec("ls -la", nil)
	if err == nil {
		t.Fatal("a command line was accepted as a program name")
	}
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("error = %v, want a classified one", err)
	}
	if e.Code != "AOS_SANDBOX_EXEC_NOT_A_PROGRAM" {
		t.Errorf("code = %q, want AOS_SANDBOX_EXEC_NOT_A_PROGRAM", e.Code)
	}
	if len(e.Actions) == 0 {
		t.Error("the model is not told how to call it instead")
	}

	// The correct shape still works.
	if _, err := box.VerifyExec("ls", []string{"-la"}); err != nil {
		t.Errorf("the two-field form was refused: %v", err)
	}
}

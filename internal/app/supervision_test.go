package app_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/env"
)

// TestTheDeliveryOfPhaseFour is the phase's claim, run against the real thing:
// two compiled binaries, one supervising the other, over a real socket.
//
// It builds them rather than calling into the packages, because "aos gateway
// start operates the daemon" is a statement about two processes and there is no
// way to check it inside one.
func TestTheDeliveryOfPhaseFour(t *testing.T) {
	if testing.Short() {
		t.Skip("builds two binaries and starts a process")
	}
	bin := t.TempDir()
	buildBinary(t, bin, "aos")
	buildBinary(t, bin, "aosd")

	home := t.TempDir()
	port := freePort(t)
	environment := append(os.Environ(),
		env.Key(env.KeyHome)+"="+home,
		env.Key(env.KeyServerPort)+"="+strconv.Itoa(port),
		env.Key(env.KeyServerHost)+"=127.0.0.1",
		env.Key("DAEMON_PATH")+"="+filepath.Join(bin, "aosd"),
		env.Key(env.KeyLogLevel)+"=error",
	)

	aos := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(filepath.Join(bin, "aos"), append(args, "--format", "json")...) //nolint:noctx // the command under test
		cmd.Env = environment
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// Nothing is running yet.
	out, err := aos(t, "gateway", "status")
	if err != nil {
		t.Fatalf("status before start: %v\n%s", err, out)
	}
	if got := statusOf(t, out); got != "stopped" {
		t.Fatalf("status = %q, want stopped\n%s", got, out)
	}

	// Start it, and make sure it is stopped whatever happens next.
	t.Cleanup(func() {
		if out, err := aos(t, "gateway", "stop"); err != nil {
			t.Logf("cleanup stop: %v\n%s", err, out)
		}
	})
	out, err = aos(t, "gateway", "start")
	if err != nil {
		t.Fatalf("start: %v\n%s", err, out)
	}
	if got := statusOf(t, out); got != "running" {
		t.Fatalf("after start, status = %q\n%s", got, out)
	}

	// Starting again is not an error and does not produce a second daemon.
	firstPID := pidOf(t, out)
	out, err = aos(t, "gateway", "start")
	if err != nil {
		t.Fatalf("second start: %v\n%s", err, out)
	}
	if got := pidOf(t, out); got != firstPID {
		t.Fatalf("a second start produced pid %d, the first was %d", got, firstPID)
	}

	// The record on disk says where it is, and the daemon is answering there.
	out, err = aos(t, "gateway", "status")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"healthy": true`) {
		t.Fatalf("the daemon is running and not answering:\n%s", out)
	}

	// And it stops on request, rather than having to be killed.
	out, err = aos(t, "gateway", "stop")
	if err != nil {
		t.Fatalf("stop: %v\n%s", err, out)
	}
	if strings.Contains(out, `"killed": true`) {
		t.Errorf("the daemon had to be killed:\n%s", out)
	}
	waitStopped(t, aos)
}

func buildBinary(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, name), "./cmd/"+name) //nolint:noctx // build step
	cmd.Dir = moduleRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

func statusOf(t *testing.T, out string) string {
	t.Helper()
	var envelope struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("output is not an envelope: %v\n%s", err, out)
	}
	return envelope.Data.Status
}

func pidOf(t *testing.T, out string) int {
	t.Helper()
	var envelope struct {
		Data struct {
			Meta struct {
				PID int `json:"pid"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("output is not an envelope: %v\n%s", err, out)
	}
	return envelope.Data.Meta.PID
}

func waitStopped(t *testing.T, aos func(*testing.T, ...string) (string, error)) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		out, err := aos(t, "gateway", "status")
		if err == nil && statusOf(t, out) == "stopped" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the daemon is still registered as running after a stop")
}

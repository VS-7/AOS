package main

import (
	"reflect"
	"testing"
)

// TestTheWindowNoticesADaemonFromAnotherRelease is the defect this guard is
// for: `aosd update apply` from a terminal replaces the daemon and the
// window's binary on disk, the watchdog adopts the restarted daemon, and the
// window that is open goes on running the previous release against it with
// nothing saying so. build.Compatible, written for exactly this, had no
// caller.
func TestTheWindowNoticesADaemonFromAnotherRelease(t *testing.T) {
	cases := []struct {
		window, daemon string
		want           *versionSkew
	}{
		{"v0.15.2-fase9", "v0.15.2-fase9", nil},
		// One release seen from two builds: a locally packaged tree.
		{"v0.15.2-fase9-dirty", "v0.15.2-fase9", nil},
		// Development builds carry no release to compare.
		{"dev", "v0.15.2-fase9", nil},
		{"v0.15.2-fase9", "dev", nil},
		// A daemon that did not say which build it is.
		{"v0.15.2-fase9", "", nil},
		{"v0.15.0-fase9", "v0.15.2-fase9", &versionSkew{
			Window: "v0.15.0-fase9", Daemon: "v0.15.2-fase9", WindowOlder: true, Compatible: true,
		}},
		{"v0.16.0", "v0.15.2-fase9", &versionSkew{
			Window: "v0.16.0", Daemon: "v0.15.2-fase9", WindowOlder: false, Compatible: false,
		}},
	}
	for _, c := range cases {
		if got := skewBetween(c.window, c.daemon); !reflect.DeepEqual(got, c.want) {
			t.Errorf("window %q, daemon %q: skew = %+v, want %+v", c.window, c.daemon, got, c.want)
		}
	}
}

// TestTheDaemonEventCarriesTheSkewOnlyWhenThereIsOne: the interface draws the
// banner from the event, so an event without it is what takes the banner down.
func TestTheDaemonEventCarriesTheSkewOnlyWhenThereIsOne(t *testing.T) {
	var got []any
	emit := func(event any) { got = append(got, event) }

	skew := &versionSkew{Window: "v0.15.0", Daemon: "v0.15.2", WindowOlder: true, Compatible: true}
	emitDaemonState(emit, true, skew)
	emitDaemonState(emit, true, nil)
	emitDaemonState(emit, false, skew)
	emitDaemonState(nil, true, skew)

	want := []any{
		map[string]any{"healthy": true, "skew": skew},
		map[string]any{"healthy": true},
		map[string]any{"healthy": false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
}

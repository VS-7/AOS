package realtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/transport/realtime"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newHub() *realtime.Hub { return realtime.NewHub(quiet(), clockx.Fixed{At: refTime}) }

// recorder is a Sink that keeps what it was sent.
type recorder struct {
	mu       sync.Mutex
	received [][]byte
	closed   bool

	// block holds Send until released, which is how a slow subscriber is
	// modelled without sleeping.
	block chan struct{}
	err   error
}

func (r *recorder) Send(_ context.Context, payload []byte) error {
	if r.block != nil {
		<-r.block
	}
	if r.err != nil {
		return r.err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, payload)
	return nil
}

func (r *recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.received)
}

// subscribe attaches a recorder and returns it with a cancel function.
func subscribe(t *testing.T, h *realtime.Hub, channel string, sink realtime.Sink) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	joined := make(chan struct{})
	go func() {
		close(joined)
		h.Subscribe(ctx, channel, sink)
	}()
	<-joined
	waitFor(t, func() bool { return h.Subscribers(channel) > 0 })
	return cancel
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition never became true")
}

func TestAnEventReachesEverySubscriberOfItsChannel(t *testing.T) {
	h := newHub()
	first, second := &recorder{}, &recorder{}
	defer subscribe(t, h, "workspace::alpha", first)()
	defer subscribe(t, h, "workspace::alpha", second)()

	elsewhere := &recorder{}
	defer subscribe(t, h, "workspace::beta", elsewhere)()

	h.Publish(context.Background(), "workspace::alpha", realtime.Event{Type: realtime.EventActivity})

	waitFor(t, func() bool { return first.count() == 1 && second.count() == 1 })
	if elsewhere.count() != 0 {
		t.Fatal("an event leaked into another workspace's channel")
	}
}

// TestEventsArriveInOrder: a streamed answer is a sequence of chunks, and one
// out of order is a corrupted sentence.
func TestEventsArriveInOrder(t *testing.T) {
	h := newHub()
	sink := &recorder{}
	defer subscribe(t, h, "c", sink)()

	const n = 50
	for i := range n {
		h.Publish(context.Background(), "c", realtime.Event{
			Type: realtime.EventChatDelta, Data: map[string]any{"i": i},
		})
	}
	waitFor(t, func() bool { return sink.count() == n })

	sink.mu.Lock()
	defer sink.mu.Unlock()
	for i, raw := range sink.received {
		var e struct {
			Data struct {
				I int `json:"i"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		if e.Data.I != i {
			t.Fatalf("message %d carries %d", i, e.Data.I)
		}
	}
}

// TestASlowSubscriberIsDroppedWithoutBlockingThePublisher is the property the
// whole bounded design exists for: the mutation that produced the event has
// already committed, and must not wait on a browser tab.
func TestASlowSubscriberIsDroppedWithoutBlockingThePublisher(t *testing.T) {
	h := newHub()
	stuck := &recorder{block: make(chan struct{})}
	fast := &recorder{}
	defer subscribe(t, h, "c", stuck)()
	defer subscribe(t, h, "c", fast)()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Far more than the queue holds, so the stuck subscriber overflows.
		for range 500 {
			h.Publish(context.Background(), "c", realtime.Event{Type: realtime.EventActivity})
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the publisher blocked on a subscriber that stopped reading")
	}

	// The one that was reading got its events.
	waitFor(t, func() bool { return fast.count() > 0 })
	// And the one that was not is gone.
	close(stuck.block)
	waitFor(t, func() bool { return h.Subscribers("c") < 2 })
}

func TestASubscriberWhoseSinkFailsIsReleased(t *testing.T) {
	h := newHub()
	broken := &recorder{err: errors.New("connection reset")}
	defer subscribe(t, h, "c", broken)()

	h.Publish(context.Background(), "c", realtime.Event{Type: realtime.EventActivity})
	waitFor(t, func() bool { return h.Subscribers("c") == 0 })

	broken.mu.Lock()
	defer broken.mu.Unlock()
	if !broken.closed {
		t.Fatal("the sink was not closed")
	}
}

func TestCancellingTheContextUnsubscribes(t *testing.T) {
	h := newHub()
	sink := &recorder{}
	cancel := subscribe(t, h, "c", sink)

	cancel()
	waitFor(t, func() bool { return h.Subscribers("c") == 0 })
}

// TestClosingTheHubReleasesEverySubscriber: on shutdown a client should see a
// closed socket rather than a silence it has to time out.
func TestClosingTheHubReleasesEverySubscriber(t *testing.T) {
	h := newHub()
	for range 3 {
		defer subscribe(t, h, "c", &recorder{})()
	}
	if h.Subscribers("c") != 3 {
		t.Fatalf("subscribers = %d", h.Subscribers("c"))
	}

	h.Close()
	waitFor(t, func() bool { return h.Subscribers("c") == 0 })

	// And a subscriber arriving after the close is turned away rather than
	// hanging forever on a hub that will never publish again.
	late := &recorder{}
	h.Subscribe(context.Background(), "c", late)
	if !late.closed {
		t.Fatal("a late subscriber was accepted by a closed hub")
	}
}

func TestAnEventWithoutATimestampGetsOne(t *testing.T) {
	h := newHub()
	sink := &recorder{}
	defer subscribe(t, h, "c", sink)()

	h.Publish(context.Background(), "c", realtime.Event{Type: realtime.EventActivity})
	waitFor(t, func() bool { return sink.count() == 1 })

	sink.mu.Lock()
	defer sink.mu.Unlock()
	var e realtime.Event
	if err := json.Unmarshal(sink.received[0], &e); err != nil {
		t.Fatal(err)
	}
	if !e.At.Equal(refTime) {
		t.Fatalf("at = %v", e.At)
	}
}

func TestChannelForIsTheOriginalsShape(t *testing.T) {
	if got := realtime.ChannelFor("project-alpha"); got != "workspace::project-alpha" {
		t.Fatalf("channel = %q", got)
	}
}

// --- the upgrade -----------------------------------------------------------

type fakeAuthorizer struct {
	allow map[string]string // workspace -> the user allowed to read it
	err   error
}

func (a fakeAuthorizer) AuthorizeWorkspace(_ context.Context, workspaceID, userID string) error {
	if a.err != nil {
		return a.err
	}
	if want, ok := a.allow[workspaceID]; ok && want == userID {
		return nil
	}
	return errors.New("forbidden")
}

// upgradeServer wires the handler behind a middleware that puts the ambient
// identity in the context, exactly as the HTTP transport does.
func upgradeServer(t *testing.T, h *realtime.Hub, auth realtime.Authorizer, origins []string) *httptest.Server {
	t.Helper()
	handler := realtime.Upgrade(realtime.Config{
		Hub: h, Auth: auth, Origins: origins, Log: quiet(),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := identity.With(r.Context(), identity.Identity{
			WorkspaceID: r.Header.Get("X-Workspace-ID"),
			UserID:      r.Header.Get("X-User-ID"),
		})
		handler(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dial(t *testing.T, srv *httptest.Server, headers http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	return websocket.Dial(t.Context(), url, &websocket.DialOptions{HTTPHeader: headers})
}

// TestACookieForAnotherWorkspaceIsRefused is defect #5. The original reads the
// workspace out of a cookie and accepts the socket without checking that the
// caller has any claim to it — so one forged line attaches you to the event
// stream of any workspace whose id you can guess.
func TestACookieForAnotherWorkspaceIsRefused(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{allow: map[string]string{"mine": "u-1"}}, nil)

	headers := http.Header{}
	headers.Set("X-Workspace-ID", "somebody-elses")
	headers.Set("X-User-ID", "u-1")

	conn, res, err := dial(t, srv, headers)
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("a socket was granted for a workspace the caller cannot read")
	}
	if res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("response = %v", res)
	}
	if h.Subscribers(realtime.ChannelFor("somebody-elses")) != 0 {
		t.Fatal("a refused caller was subscribed anyway")
	}
}

func TestASocketWithoutAWorkspaceIsRefused(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{}, nil)

	conn, res, err := dial(t, srv, http.Header{})
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("a socket was granted with no workspace named")
	}
	if res == nil || res.StatusCode != http.StatusBadRequest {
		t.Fatalf("response = %v", res)
	}
}

func TestAnAuthorisedCallerReceivesEvents(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{allow: map[string]string{"mine": "u-1"}}, nil)

	headers := http.Header{}
	headers.Set("X-Workspace-ID", "mine")
	headers.Set("X-User-ID", "u-1")

	conn, _, err := dial(t, srv, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	channel := realtime.ChannelFor("mine")
	waitFor(t, func() bool { return h.Subscribers(channel) == 1 })

	h.Publish(t.Context(), channel, realtime.Event{
		Type: realtime.EventTaskChanged, Workspace: "mine",
		Data: map[string]any{"task": "t-1"},
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	kind, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if kind != websocket.MessageText {
		t.Fatalf("message kind = %v", kind)
	}
	var e realtime.Event
	if err := json.Unmarshal(payload, &e); err != nil {
		t.Fatal(err)
	}
	if e.Type != realtime.EventTaskChanged || e.Workspace != "mine" {
		t.Fatalf("event = %+v", e)
	}
}

// TestNothingTheClientSendsIsActedOn is the property that keeps this from being
// a command surface: there is no path from an inbound frame to an action.
func TestNothingTheClientSendsIsActedOn(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{allow: map[string]string{"mine": "u-1"}}, nil)

	headers := http.Header{}
	headers.Set("X-Workspace-ID", "mine")
	headers.Set("X-User-ID", "u-1")
	conn, _, err := dial(t, srv, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	channel := realtime.ChannelFor("mine")
	waitFor(t, func() bool { return h.Subscribers(channel) == 1 })

	// Anything at all, including something that looks like a command.
	if err := conn.Write(t.Context(), websocket.MessageText,
		[]byte(`{"action":"gateway_stop","input":{}}`)); err != nil {
		t.Fatal(err)
	}

	// The socket stays open and nothing comes back but what the hub sends.
	h.Publish(t.Context(), channel, realtime.Event{Type: realtime.EventActivity})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var e realtime.Event
	if err := json.Unmarshal(payload, &e); err != nil {
		t.Fatal(err)
	}
	if e.Type != realtime.EventActivity {
		t.Fatalf("the socket answered an inbound frame: %+v", e)
	}
}

// TestAnUnlistedOriginIsRefused: the daemon runs on the machine the browser
// runs on, so without this any page the person opens can attach to the stream.
func TestAnUnlistedOriginIsRefused(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{allow: map[string]string{"mine": "u-1"}},
		[]string{"localhost:5173"})

	headers := http.Header{}
	headers.Set("X-Workspace-ID", "mine")
	headers.Set("X-User-ID", "u-1")
	headers.Set("Origin", "https://evil.example")

	conn, _, err := dial(t, srv, headers)
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("a socket was granted to an unlisted origin")
	}
}

func TestClosingTheSocketUnsubscribes(t *testing.T) {
	h := newHub()
	srv := upgradeServer(t, h, fakeAuthorizer{allow: map[string]string{"mine": "u-1"}}, nil)

	headers := http.Header{}
	headers.Set("X-Workspace-ID", "mine")
	headers.Set("X-User-ID", "u-1")
	conn, _, err := dial(t, srv, headers)
	if err != nil {
		t.Fatal(err)
	}
	channel := realtime.ChannelFor("mine")
	waitFor(t, func() bool { return h.Subscribers(channel) == 1 })

	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return h.Subscribers(channel) == 0 })
}

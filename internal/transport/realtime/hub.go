// Package realtime is the server-push channel: workspace events, streamed
// agent output, and requests for approval.
//
// It is one-directional by design, and that is a security property rather than
// a simplification. A socket that accepts commands needs authorisation per
// message; a socket that only emits needs it once, at the upgrade. Every
// mutation goes over HTTP, including the answer to an approval request.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
)

// Event is one message pushed to a subscriber.
type Event struct {
	Type      string    `json:"type"`
	Workspace string    `json:"workspace,omitempty"`
	At        time.Time `json:"at"`
	Data      any       `json:"data,omitempty"`
}

// The event types the system publishes. They are constants because the desktop
// switches on them, and a typo in a string literal on either side is a bug that
// shows up as silence.
const (
	EventActivity          = "activity"           // a new entry in the inbox
	EventChatDelta         = "chat.delta"         // a chunk of a streamed answer
	EventChatDone          = "chat.done"          // the turn ended, with its token usage
	EventTaskChanged       = "task.changed"       // a task moved
	EventApprovalRequest   = "approval.request"   // a tool is waiting for a decision
	EventCollectionChanged = "collection.changed" // the watcher saw a file change
)

// Types lists every event this channel emits.
var Types = []string{
	EventActivity, EventChatDelta, EventChatDone,
	EventTaskChanged, EventApprovalRequest, EventCollectionChanged,
}

// Sink is one subscriber's end of the channel.
type Sink interface {
	// Send delivers one message. It must return promptly: the hub holds no
	// lock while calling it, but a Send that blocks holds up that subscriber's
	// queue and eventually gets it dropped.
	Send(ctx context.Context, payload []byte) error

	// Close releases the subscriber.
	Close() error
}

// queueDepth is how far behind a subscriber may fall before it is dropped.
//
// The number is small on purpose. A browser tab that is not reading is not
// going to catch up, and buffering more of the past for it costs memory that
// the subscribers who are reading need.
const queueDepth = 64

// Hub fans events out to the subscribers of a workspace channel.
//
// Delivery is bounded and a slow subscriber is dropped rather than allowed to
// block. The mutation that produced an event has already completed and is not
// going to wait on a browser: the alternative to dropping is a write path whose
// latency is set by its slowest reader.
type Hub struct {
	log   *slog.Logger
	clock clockx.Clock

	mu     sync.RWMutex
	byChan map[string]map[*subscriber]struct{}
	closed bool
}

// NewHub builds an empty hub. A nil clock reads the wall clock.
func NewHub(log *slog.Logger, clock clockx.Clock) *Hub {
	if log == nil {
		log = slog.Default()
	}
	if clock == nil {
		clock = clockx.System{}
	}
	return &Hub{log: log, clock: clock, byChan: map[string]map[*subscriber]struct{}{}}
}

type subscriber struct {
	sink    Sink
	channel string
	queue   chan []byte

	once sync.Once
	done chan struct{}
}

// ChannelFor is the name of a workspace's channel.
func ChannelFor(workspaceID string) string { return "workspace::" + workspaceID }

// Subscribe attaches a sink to a channel and pumps events to it until the
// context ends, the sink fails, or the subscriber falls too far behind.
//
// It blocks. The caller is an HTTP handler whose lifetime is the connection's,
// which is exactly the lifetime of the subscription.
func (h *Hub) Subscribe(ctx context.Context, channel string, sink Sink) {
	sub := &subscriber{
		sink: sink, channel: channel,
		queue: make(chan []byte, queueDepth),
		done:  make(chan struct{}),
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = sink.Close()
		return
	}
	if h.byChan[channel] == nil {
		h.byChan[channel] = map[*subscriber]struct{}{}
	}
	h.byChan[channel][sub] = struct{}{}
	count := len(h.byChan[channel])
	h.mu.Unlock()

	h.log.Debug("subscriber joined", "channel", channel, "subscribers", count)
	defer h.remove(sub)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.done:
			return
		case payload := <-sub.queue:
			if err := sink.Send(ctx, payload); err != nil {
				h.log.Debug("subscriber went away", "channel", channel, "err", err)
				return
			}
		}
	}
}

// Publish delivers an event to every subscriber of a channel.
//
// It never blocks. A subscriber whose queue is full is dropped, because the
// caller is a write path that has already committed and must not have its
// latency set by a browser tab nobody is looking at.
func (h *Hub) Publish(_ context.Context, channel string, e Event) {
	if e.At.IsZero() {
		// A timestamp is what lets a client tell a replayed event from a live
		// one, so an event without one is not publishable.
		e.At = h.clock.Now()
	}
	payload, err := json.Marshal(e)
	if err != nil {
		h.log.Error("could not encode an event", "type", e.Type, "err", err)
		return
	}

	h.mu.RLock()
	subs := make([]*subscriber, 0, len(h.byChan[channel]))
	for s := range h.byChan[channel] {
		subs = append(subs, s)
	}
	h.mu.RUnlock()

	var dropped int
	for _, s := range subs {
		select {
		case s.queue <- payload:
		default:
			dropped++
			s.drop()
		}
	}
	if dropped > 0 {
		h.log.Warn("dropped subscribers that were not keeping up",
			"channel", channel, "dropped", dropped, "type", e.Type)
	}
}

// Subscribers reports how many are attached to a channel.
func (h *Hub) Subscribers(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byChan[channel])
}

// Close releases every subscriber. It is called when the daemon shuts down, so
// that clients see a closed socket rather than a silence they have to time out.
func (h *Hub) Close() {
	h.mu.Lock()
	h.closed = true
	all := h.byChan
	h.byChan = map[string]map[*subscriber]struct{}{}
	h.mu.Unlock()

	for _, subs := range all {
		for s := range subs {
			s.drop()
		}
	}
}

func (h *Hub) remove(s *subscriber) {
	h.mu.Lock()
	if subs := h.byChan[s.channel]; subs != nil {
		delete(subs, s)
		if len(subs) == 0 {
			delete(h.byChan, s.channel)
		}
	}
	h.mu.Unlock()
	_ = s.sink.Close()
}

func (s *subscriber) drop() {
	s.once.Do(func() { close(s.done) })
}

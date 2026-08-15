package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/aos/internal/core/identity"
)

// Authorizer decides whether the caller may read a workspace's events.
//
// It is a port with one method because that is the whole question, and because
// the answer has to be given before the socket is accepted — see Upgrade.
type Authorizer interface {
	AuthorizeWorkspace(ctx context.Context, workspaceID, userID string) error
}

// Config is what the upgrade handler is built from.
type Config struct {
	Hub  *Hub
	Auth Authorizer

	// Origins is the list of browser origins allowed to open a socket. It is
	// explicit rather than disabled: the daemon runs on the machine the browser
	// runs on, so without it any page the person opens can attach to the event
	// stream of their workspace.
	Origins []string

	// WriteTimeout bounds one write to a subscriber. A client that stops
	// reading stops the write, which is what gets it dropped.
	WriteTimeout time.Duration

	Log *slog.Logger
}

// defaultWriteTimeout is short: it is the deadline for handing bytes to a
// socket, not for anything the client has to do with them.
const defaultWriteTimeout = 5 * time.Second

// Upgrade authorises, then accepts.
//
// The order is the fix for defect #5. The original reads the workspace out of a
// cookie and accepts the socket, without checking that the caller has any claim
// to that workspace — so forging one line of a cookie attaches you to the event
// stream of any workspace whose id you can guess.
func Upgrade(cfg Config) http.HandlerFunc {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = defaultWriteTimeout
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ambient := identity.From(r.Context())
		if ambient.WorkspaceID == "" {
			http.Error(w, "a workspace must be named", http.StatusBadRequest)
			return
		}
		if cfg.Auth == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := cfg.Auth.AuthorizeWorkspace(r.Context(), ambient.WorkspaceID, ambient.UserID); err != nil {
			log.Warn("refused a socket for a workspace the caller cannot read",
				"workspace", ambient.WorkspaceID, "user", ambient.UserID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: cfg.Origins,
		})
		if err != nil {
			// Accept has already written the response.
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// The socket carries nothing inbound. Reading anyway is not optional:
		// a connection nobody reads from never notices a close, and the
		// library needs the read loop running to answer pings.
		go drain(ctx, cancel, conn)

		cfg.Hub.Subscribe(ctx, ChannelFor(ambient.WorkspaceID), &socket{
			conn: conn, timeout: writeTimeout,
		})
	}
}

// drain reads and discards. Anything a client sends is ignored, which is the
// property that keeps this socket from being a command surface: there is no
// code path from an inbound frame to an action.
func drain(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn) {
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

// socket is the Sink over a websocket connection.
type socket struct {
	conn    *websocket.Conn
	timeout time.Duration
}

func (s *socket) Send(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageText, payload)
}

func (s *socket) Close() error {
	return s.conn.Close(websocket.StatusNormalClosure, "")
}

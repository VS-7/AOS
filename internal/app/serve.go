package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/transport/authapi"
	"github.com/OWNER/aos/internal/transport/fileapi"
	"github.com/OWNER/aos/internal/transport/httpapi"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// ServeOptions select where the daemon listens.
type ServeOptions struct {
	Host string
	Port int

	// ShutdownTimeout bounds the wait for in-flight requests on shutdown.
	ShutdownTimeout time.Duration

	Log *slog.Logger

	// Ready is closed once the listener is open. It exists for the test that
	// starts a daemon and needs to know when to talk to it, rather than
	// sleeping and hoping.
	Ready func(addr string)
}

// Serve runs the HTTP daemon until the context ends.
//
// Shutdown is graceful and bounded: in-flight requests get the timeout to
// finish, and then the process stops waiting. A daemon that hangs on shutdown
// is a daemon the supervisor has to kill, which is how work gets lost.
func (a *App) Serve(ctx context.Context, opts ServeOptions) error {
	resolver := a.env
	host := opts.Host
	if host == "" {
		host = resolver.String(env.KeyServerHost, env.DefaultServerHost)
	}
	port := opts.Port
	if port == 0 {
		port = resolver.Int(env.KeyServerPort, build.Port)
	}
	timeout := opts.ShutdownTimeout
	if timeout == 0 {
		timeout = resolver.Duration(env.KeyShutdownTimeout, env.DefaultShutdownTimeout)
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	current, err := a.Config.Raw(ctx)
	if err != nil {
		return err
	}
	if err := guardExposure(host, current); err != nil {
		return err
	}

	origins := allowedOrigins(resolver)
	server := httpapi.New(httpapi.Config{
		Registry:   a.Registry,
		Auth:       a.Auth,
		Files:      fileapi.New(fileapi.Config{Service: a.Files, Log: log}),
		AuthRoutes: authapi.New(authapi.Config{Service: a.Auth, Log: log, Clock: a.Clock}),
		Realtime: realtime.Upgrade(realtime.Config{
			Hub:     a.Events,
			Auth:    a.Workspaces,
			Origins: origins,
			Log:     log,
		}),
		// Read per request rather than captured: the configuration file is
		// meant to be edited while the daemon runs.
		SecurityEnabled: func() bool {
			c, err := a.Config.Raw(ctx)
			if err != nil {
				// Unreadable configuration means the safe reading, not the
				// convenient one.
				return true
			}
			return c.Security.Enabled
		},
		DocsEnabled:    !resolver.IsProduction(),
		AllowedOrigins: origins,
		Log:            log,
	})

	// Only the daemon runs the watcher: a one-shot CLI process exits before a
	// long-lived goroutine could ever do anything, and starting one anyway
	// would just be a leaked fsnotify handle. Tied to ctx, it stops itself on
	// the same shutdown Serve already waits for.
	if a.Watcher != nil {
		go func() {
			if err := a.Watcher.Watch(ctx); err != nil {
				log.Warn("collection watcher stopped", "err", err)
			}
		}()
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	// Through a ListenConfig so the context governs the bind too: a daemon
	// asked to stop while it is still opening its socket should stop.
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return errCannotListen(addr, err)
	}
	if opts.Ready != nil {
		opts.Ready(listener.Addr().String())
	}
	log.Info("serving", "addr", listener.Addr().String(), "version", build.Version)

	srv := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errs := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", timeout.String())
	// Subscribers see a closed socket rather than a silence they have to time
	// out on.
	a.Events.Close()
	// Not the cancelled context: shutting down needs a live one, or the
	// graceful wait ends immediately and the whole point is lost.
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := srv.Shutdown(stopCtx); err != nil {
		log.Warn("in-flight requests did not finish in time", "err", err)
		_ = srv.Close()
	}
	return <-errs
}

// guardExposure refuses to serve beyond loopback without authentication.
//
// This is ADR-0009, and it is enforced here rather than per request because it
// is a property of the installation and not of a call. The original binds
// 0.0.0.0 by default (defect #2); binding wide is allowed here, and doing it
// with the door open is not.
func guardExposure(host string, c config.Config) error {
	if c.Security.Enabled {
		return nil
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errExposedWithoutAuth(host)
}

// allowedOrigins is the CORS allowlist. The desktop app's development server is
// the only browser origin that exists before phase 7.
func allowedOrigins(r *env.Resolver) []string {
	raw := r.String("ALLOWED_ORIGINS", "")
	if raw == "" {
		return nil
	}
	return splitAndTrim(raw)
}

func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func errCannotListen(addr string, cause error) error {
	return apperr.New("SERVER_CANNOT_LISTEN").
		Causer("app.Serve").
		Msgf("cannot listen on %s", addr).
		Issue("addr", addr).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label:   "something may already be listening there — ask what the gateway thinks",
			Command: build.Name + " gateway status",
			Tool:    "gateway_status",
		})
}

func errExposedWithoutAuth(host string) error {
	return apperr.New("SERVER_EXPOSED_WITHOUT_AUTH").
		Causer("app.guardExposure").
		Msgf("refusing to listen on %s with authentication switched off", host).
		Issue("host", host).
		Status(apperr.StatusBadRequest).
		CTA(
			apperr.CallToAction{
				Label:   "turn authentication on before exposing the daemon",
				Command: build.Name + " config update --set security.enabled=true",
				Tool:    "config_update",
				Input:   map[string]any{"set": map[string]any{"security.enabled": true}},
			},
			apperr.CallToAction{
				Label: "or bind to " + env.DefaultServerHost + ", which is the default",
			},
		)
}

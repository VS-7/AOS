// Package telegramapi is the Telegram implementation of bot.Provider, over
// plain net/http — no SDK, the same choice internal/runtime/providers makes
// for the LLM adapters, for the identical reason: the surface this package
// actually needs (setWebhook, deleteWebhook, sendMessage, sendChatAction) is
// a handful of calls, and an SDK's dependency tree buys coverage of the rest
// of the Bot API this build has no use for.
package telegramapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/domain/bot"
)

const defaultBaseURL = "https://api.telegram.org"

// maxChars and maxBlocks are the design doc's own limits on one logical
// message before Send must split it — see split.go.
const (
	maxChars  = 32768
	maxBlocks = 500
)

// secretHeader is the header Telegram echoes back the secret_token
// setWebhook was given on — see https://core.telegram.org/bots/api#setwebhook.
const secretHeader = "X-Telegram-Bot-Api-Secret-Token"

// Provider is the Telegram bot.Provider. One instance serves every agent's
// channel — see bot.Provider's own doc on why Send/SetTyping take a token
// per call rather than one baked in at construction.
type Provider struct {
	baseURL string
	hc      *http.Client
}

// New builds a Provider against the real Telegram Bot API.
func New() *Provider { return &Provider{baseURL: defaultBaseURL, hc: http.DefaultClient} }

// NewWithClient builds a Provider against baseURL with hc — the seam a test
// uses to point at an httptest.Server instead of the real API.
func NewWithClient(baseURL string, hc *http.Client) *Provider {
	return &Provider{baseURL: strings.TrimRight(baseURL, "/"), hc: hc}
}

func (p *Provider) Name() string { return "telegram" }

func (p *Provider) method(token, name string) string {
	return p.baseURL + "/bot" + token + "/" + name
}

func (p *Provider) call(ctx context.Context, token, method string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return errEncodeFailed(method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.method(token, method), bytes.NewReader(raw))
	if err != nil {
		return errRequestFailed(method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.hc.Do(req)
	if err != nil {
		return errRequestFailed(method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errRequestFailed(method, err)
	}
	var apiResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return errAPIFailed(method, resp.StatusCode, string(respBody))
	}
	if !apiResp.OK {
		return errAPIFailed(method, resp.StatusCode, apiResp.Description)
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (p *Provider) RegisterWebhook(ctx context.Context, url, secret string, cfg map[string]any) error {
	token, _ := cfg["token"].(string)
	return p.call(ctx, token, "setWebhook", map[string]any{
		"url":          url,
		"secret_token": secret,
	}, nil)
}

func (p *Provider) UnregisterWebhook(ctx context.Context, cfg map[string]any) error {
	token, _ := cfg["token"].(string)
	return p.call(ctx, token, "deleteWebhook", map[string]any{}, nil)
}

// update is the slice of Telegram's Update object this package reads.
type update struct {
	Message *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"message"`
}

// Parse verifies secret in constant time before reading anything Telegram
// sent — a request whose secret does not match is refused unconditionally,
// even for a body that would otherwise parse fine.
func (p *Provider) Parse(ctx context.Context, r *http.Request, secret string) (bot.Inbound, error) {
	got := r.Header.Get(secretHeader)
	if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		return bot.Inbound{}, errWebhookSecretMismatch()
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return bot.Inbound{}, errRequestFailed("parse update", err)
	}
	var u update
	if err := json.Unmarshal(raw, &u); err != nil {
		return bot.Inbound{}, errDecodeFailed(err)
	}
	if u.Message == nil {
		return bot.Inbound{}, errUnsupportedUpdate()
	}
	return bot.Inbound{
		ChatID: fmt.Sprintf("%d", u.Message.Chat.ID),
		UserID: fmt.Sprintf("%d", u.Message.From.ID),
		Text:   u.Message.Text,
	}, nil
}

// Send delivers out, splitting into as many sendMessage calls as split
// requires. A failure partway through is reported once, naming how many
// chunks made it — a caller cannot retry the whole message without risking a
// duplicate of what already sent.
func (p *Provider) Send(ctx context.Context, token string, out bot.Outbound) error {
	chunks := split(out.Text, maxChars)
	for i, c := range chunks {
		if err := p.call(ctx, token, "sendMessage", map[string]any{
			"chat_id": out.ChatID,
			"text":    c,
		}, nil); err != nil {
			return errSendPartial(i, len(chunks), err)
		}
	}
	return nil
}

func (p *Provider) SetTyping(ctx context.Context, token, chatID string, on bool) error {
	if !on {
		return nil // Telegram's typing indicator has no "off" — it just expires.
	}
	return p.call(ctx, token, "sendChatAction", map[string]any{
		"chat_id": chatID,
		"action":  "typing",
	}, nil)
}

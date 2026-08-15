// Package fake is a scripted model provider.
//
// It is what makes the loop testable at all. A turn is a conversation between
// the loop and a model, and the only way to test the conversation is to control
// both ends: the script says what the model answers on each step, and the test
// asserts on what the loop did with it. Every behaviour of the loop — blocking,
// rewriting, denial, parallelism, compaction, the ceilings — is proved against
// this and no network.
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// Step is one scripted answer.
type Step struct {
	// Text is the assistant's answer. A step with tool calls and no text is
	// the ordinary shape of a model that wants to act before speaking.
	Text      string
	Reasoning string

	// Calls are the tools the model asks for. More than one in a step is the
	// parallel case the master prompt asks for.
	Calls []agentloop.ToolCall

	Usage      agentloop.Usage
	StopReason string

	// Err makes this step fail, which is how a provider outage is tested.
	Err error
}

// Provider answers from a script.
type Provider struct {
	// Script is consumed one step per model call. Running past the end is a
	// test bug rather than a silent empty answer, so it produces an error that
	// says which step was missing.
	Script []Step

	// Name is what the provider calls itself.
	ProviderName string

	mu       sync.Mutex
	step     int
	requests []agentloop.Request
}

// Text is the shorthand for a provider that answers once and stops.
func Text(answer string) *Provider {
	return &Provider{Script: []Step{{Text: answer, StopReason: agentloop.StopEnd}}}
}

// Call builds a tool call for a script.
func Call(id, name string, input any) agentloop.ToolCall {
	raw, err := json.Marshal(input)
	if err != nil {
		panic(fmt.Sprintf("fake: the scripted input for %s cannot be encoded: %v", name, err))
	}
	return agentloop.ToolCall{ID: id, Name: name, Input: raw}
}

// Name reports the provider's name.
func (p *Provider) Name() string {
	if p.ProviderName == "" {
		return "fake"
	}
	return p.ProviderName
}

// Requests returns what the loop asked for, in order. It is how a test asserts
// on the prompt the loop built rather than only on the answer it got.
func (p *Provider) Requests() []agentloop.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]agentloop.Request(nil), p.requests...)
}

// Steps reports how many model calls were made.
func (p *Provider) Steps() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.step
}

// Generate answers the next scripted step.
func (p *Provider) Generate(_ context.Context, req agentloop.Request) (agentloop.Response, error) {
	step, err := p.next(req)
	if err != nil {
		return agentloop.Response{}, err
	}
	return responseOf(step, req.Model), nil
}

// Stream answers the same step, one word at a time, so a test can assert that
// the deltas add up to the answer.
func (p *Provider) Stream(_ context.Context, req agentloop.Request) (agentloop.Stream, error) {
	step, err := p.next(req)
	if err != nil {
		return nil, err
	}
	return &stream{step: step, resp: responseOf(step, req.Model), words: split(step.Text)}, nil
}

func (p *Provider) next(req agentloop.Request) (Step, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)

	if p.step >= len(p.Script) {
		// A loop that asks for more than the script has is a loop that did not
		// stop when the test expected it to, and saying so beats an empty
		// answer that makes it look like it did.
		return Step{}, fmt.Errorf("fake: the script has %d steps and the loop asked for step %d",
			len(p.Script), p.step+1)
	}
	step := p.Script[p.step]
	p.step++
	if step.Err != nil {
		return Step{}, step.Err
	}
	return step, nil
}

func responseOf(s Step, model string) agentloop.Response {
	stop := s.StopReason
	if stop == "" {
		if len(s.Calls) > 0 {
			stop = agentloop.StopToolCalls
		} else {
			stop = agentloop.StopEnd
		}
	}
	usage := s.Usage
	if usage.Total == 0 {
		usage.Total = usage.Input + usage.Output + usage.Reasoning
	}
	return agentloop.Response{
		Message: agentloop.Message{
			Role: agentloop.RoleAssistant, Text: s.Text,
			Reasoning: s.Reasoning, ToolCalls: s.Calls,
		},
		ToolCalls:  s.Calls,
		Usage:      usage,
		StopReason: stop,
		Model:      model,
	}
}

type stream struct {
	step  Step
	resp  agentloop.Response
	words []string
	i     int
	sent  bool
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	if !s.sent && s.step.Reasoning != "" {
		s.sent = true
		return agentloop.Chunk{Reasoning: s.step.Reasoning}, nil
	}
	s.sent = true
	if s.i >= len(s.words) {
		return agentloop.Chunk{}, io.EOF
	}
	word := s.words[s.i]
	s.i++
	return agentloop.Chunk{Text: word}, nil
}

func (s *stream) Response() agentloop.Response { return s.resp }
func (s *stream) Close() error                 { return nil }

// split keeps the separators so that joining the chunks reproduces the answer
// exactly. A stream whose pieces do not add up to the whole is a stream that
// shows the user something different from what was said.
func split(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.SplitAfter(text, " ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

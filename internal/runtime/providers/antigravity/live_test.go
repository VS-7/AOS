package antigravity_test

// The live check. It talks to Google with this machine's own Antigravity
// login, so it does not run unless somebody asks for it:
//
//	AOS_ANTIGRAVITY_LIVE=1 go test ./internal/runtime/providers/antigravity/ -run Live -v
//
// It exists because the recorded contract beside it proves the adapter reads
// what the service documents, not that the service still sends it — and this
// endpoint is an internal one with no published contract to hold it to. When
// this fails and the recorded suite passes, the wire format moved.
//
// It spends a small amount of a real allowance: two short turns on the
// cheapest model the catalogue offers.

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/antigravity"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

func liveProvider(t *testing.T) *antigravity.Provider {
	t.Helper()
	if os.Getenv("AOS_ANTIGRAVITY_LIVE") != "1" {
		t.Skip("set AOS_ANTIGRAVITY_LIVE=1 to call Google with this machine's login")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return antigravity.New(providers.Config{Home: home})
}

func TestLiveTheServiceStillPublishesACatalogue(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	models, err := p.Models(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("the catalogue is empty")
	}
	for _, m := range models {
		t.Logf("%-30s %s", m.ID, m.Name)
	}
}

func TestLiveATurnWithAToolCompletes(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	schema, err := jsonschema.For[struct {
		City string `json:"city" jsonschema:"Which city."`
	}](nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := []toolexec.Spec{{
		Name: "get_weather", Description: "Current weather for a city.", InputSchema: schema,
	}}

	first, err := p.Generate(ctx, agentloop.Request{
		Model:        liveModel,
		Instructions: "You are terse. Use the tool when asked about weather.",
		Reasoning:    agentloop.ReasoningLow,
		Tools:        tools,
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "What is the weather in Recife? Use the tool."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("the model asked for %d tools: %+v (text %q)", len(first.ToolCalls), first.ToolCalls, first.Message.Text)
	}
	call := first.ToolCalls[0]
	t.Logf("tool call: id=%q name=%q args=%s signature=%d bytes", call.ID, call.Name, call.Input, len(call.Signature))
	if call.Name != "get_weather" {
		t.Errorf("name = %q", call.Name)
	}
	if first.StopReason != agentloop.StopToolCalls {
		t.Errorf("stop = %q", first.StopReason)
	}
	if first.Usage.Total == 0 {
		t.Error("no usage was reported")
	}

	// The second half is the one that regresses: a follow-up whose function
	// call lost its thought signature is refused outright.
	second, err := p.Generate(ctx, agentloop.Request{
		Model:        liveModel,
		Instructions: "You are terse. One short sentence.",
		Reasoning:    agentloop.ReasoningLow,
		Tools:        tools,
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "What is the weather in Recife? Use the tool."},
			{Role: agentloop.RoleAssistant, ToolCalls: first.ToolCalls},
			{Role: agentloop.RoleTool, CallID: call.ID, Name: call.Name,
				Result: []byte(`{"tempC":31,"sky":"sunny"}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("answer: %q (usage %+v)", second.Message.Text, second.Usage)
	if second.Message.Text == "" {
		t.Fatal("the follow-up produced no text")
	}
	if second.StopReason != agentloop.StopEnd {
		t.Errorf("stop = %q", second.StopReason)
	}
}

func TestLiveAStreamArrivesInPieces(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	st, err := p.Stream(ctx, agentloop.Request{
		Model:        liveModel,
		Instructions: "Be concise.",
		Reasoning:    agentloop.ReasoningMedium,
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "Reply with exactly: the readme says hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	var text, thinking strings.Builder
	var chunks int
	for {
		chunk, err := st.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		chunks++
		text.WriteString(chunk.Text)
		thinking.WriteString(chunk.Reasoning)
	}
	final := st.Response()
	t.Logf("%d chunks, %d chars of text, %d of reasoning, usage %+v",
		chunks, text.Len(), thinking.Len(), final.Usage)
	if text.Len() == 0 {
		t.Fatal("the stream carried no text")
	}
	if strings.TrimSpace(final.Message.Text) != strings.TrimSpace(text.String()) {
		t.Fatalf("the stream said %q and the answer is %q", text.String(), final.Message.Text)
	}
}

// liveModel is the cheapest entry the catalogue offers that still thinks and
// calls tools, so the check costs as little of a real allowance as it can.
const liveModel = "gemini-3.6-flash-low"

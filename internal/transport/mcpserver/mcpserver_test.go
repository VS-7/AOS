package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

type storeInput struct {
	Title      string  `json:"title" jsonschema:"A brief summary of the memory content." validate:"required"`
	Confidence float64 `json:"confidence,omitempty" jsonschema:"0..1 — how sure the caller is."`

	command.Reasoning
}

type storeOutput struct {
	ID string `json:"id"`
}

type calls struct{ store, recall int }

func newRegistry(t *testing.T) (*command.Registry, *calls) {
	t.Helper()
	counter := &calls{}
	reg := command.NewRegistry()

	must(t, command.Register(reg, command.Command[storeInput, storeOutput]{
		Group: "memories", Name: "store",
		Summary: "Record a durable memory.",
		Doc:     "Record a durable memory in the knowledge graph.",
		Examples: []command.Example{
			{Description: "a preference", Input: storeInput{Title: "commits in English"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Store a memory", IdempotentHint: true},
		Handler: func(_ context.Context, in storeInput) (storeOutput, error) {
			counter.store++
			return storeOutput{ID: "m-1"}, nil
		},
	}))

	must(t, command.Register(reg, command.Command[storeInput, storeOutput]{
		Group: "memories", Name: "recall",
		Summary:     "Search the memory graph.",
		Doc:         "Search the memory graph and return what is relevant.",
		Registry:    true,
		Annotations: command.Annotations{Title: "Recall memories", ReadOnlyHint: true, IdempotentHint: true},
		Handler: func(_ context.Context, in storeInput) (storeOutput, error) {
			counter.recall++
			return storeOutput{ID: "m-2"}, nil
		},
	}))

	// Outside the agent registry: the privilege boundary.
	must(t, command.Register(reg, command.Command[storeInput, storeOutput]{
		Group: "gateway", Name: "restart",
		Summary:  "Restart the daemon.",
		Registry: false,
		Handler: func(context.Context, storeInput) (storeOutput, error) {
			return storeOutput{}, nil
		},
	}))

	reg.DescribeGroup(command.GroupDoc{
		Name: "memories", Tool: "Memory",
		Summary: "Remember and recall.",
		Doc:     "The durable knowledge of the workspace.",
		Hint:    "Recall before you store.",
	})
	reg.DescribeGroup(command.GroupDoc{Name: "gateway", Tool: "Gateway", Summary: "Daemon lifecycle."})
	return reg, counter
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// connect wires a real client to a real server over the in-memory transport, so
// the tests exercise the protocol rather than the handler function.
func connect(t *testing.T, cfg mcpserver.Config) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcpserver.New(cfg)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		_ = serverSession.Wait()
	})
	return session
}

func listTools(t *testing.T, session *mcp.ClientSession) []*mcp.Tool {
	t.Helper()
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Tools
}

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestFlatShapePublishesOneToolPerCommand, with the name being the command path
// joined by underscores and the list alphabetical.
func TestFlatShapePublishesOneToolPerCommand(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})

	var names []string
	for _, tool := range listTools(t, session) {
		names = append(names, tool.Name)
	}
	want := []string{"gateway_restart", "memories_recall", "memories_store"}
	if len(names) != len(want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tools = %v, want %v (alphabetical and stable)", names, want)
		}
	}
}

// TestPublishedSchemaCarriesTheReasoningField is the one the whole tool surface
// depends on: the inference library drops the fields of an embedded struct, so
// without the repair at registration no published schema would mention the
// field every call must carry.
func TestPublishedSchemaCarriesTheReasoningField(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})

	for _, tool := range listTools(t, session) {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema.Properties[command.ReasoningField]; !ok {
			t.Errorf("%s publishes a schema without %s: %s", tool.Name, command.ReasoningField, raw)
		}
		found := false
		for _, r := range schema.Required {
			if r == command.ReasoningField {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not mark %s required", tool.Name, command.ReasoningField)
		}
	}
}

func TestAnnotationsTravelToTheClient(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})

	for _, tool := range listTools(t, session) {
		if tool.Name != "memories_recall" {
			continue
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("recall must be announced as read-only: %+v", tool.Annotations)
		}
		return
	}
	t.Fatal("memories_recall was not published")
}

func TestCompositeShapePublishesOneToolPerGroup(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	tools := listTools(t, session)
	if len(tools) != 2 {
		var names []string
		for _, tool := range tools {
			names = append(names, tool.Name)
		}
		t.Fatalf("tools = %v, want one per group", names)
	}
	var memory *mcp.Tool
	for _, tool := range tools {
		if tool.Name == "Memory" {
			memory = tool
		}
	}
	if memory == nil {
		t.Fatal("the memories group did not become the Memory tool")
	}

	// The description is assembled the way the original assembles it.
	for _, want := range []string{
		"The durable knowledge of the workspace.",
		"Composite tool `Memory` with 2 actions: recall, store.",
		"Recall before you store.",
		"## Usage",
		"Call as `Memory({ action: \"<action>\", input: { ... }, _reasoning: \"...\" })`.",
		"Set `schema: true` on the same level as `action`",
	} {
		if !strings.Contains(memory.Description, want) {
			t.Errorf("the description is missing %q:\n%s", want, memory.Description)
		}
	}
}

// TestCompositeMergesAnnotations: read-only only if every action is,
// destructive if any action is.
func TestCompositeMergesAnnotations(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})
	for _, tool := range listTools(t, session) {
		if tool.Name != "Memory" {
			continue
		}
		if tool.Annotations.ReadOnlyHint {
			t.Error("a group with a writing action is not read-only")
		}
		return
	}
	t.Fatal("Memory was not published")
}

func TestCompositeRoutesToTheAction(t *testing.T) {
	reg, counter := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Memory",
		Arguments: mustJSON(t, mcpserver.CompositeInput{
			Action:    "store",
			Input:     mustJSON(t, storeInput{Title: "a memory"}),
			Reasoning: "recording a decision",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("call failed: %s", text(t, res))
	}
	if counter.store != 1 || counter.recall != 0 {
		t.Fatalf("routing is wrong: %+v", counter)
	}
	if !strings.Contains(text(t, res), `"m-1"`) {
		t.Errorf("result = %s", text(t, res))
	}
}

// TestSchemaTrueInspectsWithoutExecuting is what makes introspection usable for
// a destructive action: the master prompt tells the agent to read the contract
// before calling, and a tool that ran anyway would make that advice dangerous.
func TestSchemaTrueInspectsWithoutExecuting(t *testing.T) {
	reg, counter := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Memory",
		Arguments: mustJSON(t, mcpserver.CompositeInput{
			Action: "store", Schema: true, Reasoning: "inspecting before calling",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("call failed: %s", text(t, res))
	}
	if counter.store != 0 {
		t.Fatal("schema:true executed the action")
	}

	var envelope struct {
		Data mcpserver.ActionDetail `json:"data"`
	}
	if err := json.Unmarshal([]byte(text(t, res)), &envelope); err != nil {
		t.Fatalf("detail is not an envelope: %v\n%s", err, text(t, res))
	}
	detail := envelope.Data
	if detail.Tool != "Memory" || detail.Action != "store" {
		t.Errorf("detail = %+v", detail)
	}
	if detail.Description == "" || detail.InputSchema == nil {
		t.Errorf("the detail must carry the description and the input schema: %+v", detail)
	}
	if len(detail.Examples) != 1 {
		t.Errorf("the examples must travel with the detail: %+v", detail.Examples)
	}
	if detail.Tokens.Total <= 0 || detail.Tokens.InputSchema <= 0 {
		t.Errorf("the agent cannot budget without an estimate: %+v", detail.Tokens)
	}
}

// TestCompositeCarriesReasoningOutsideTheActionInput is the shape a real client
// sends: the per-action schema has no `_reasoning`, because the field belongs
// to the composite payload. The handler splices the two back together.
func TestCompositeCarriesReasoningOutsideTheActionInput(t *testing.T) {
	reg, counter := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Memory",
		Arguments: json.RawMessage(`{
			"action": "store",
			"input": {"title": "a memory"},
			"_reasoning": "recording a decision"
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("the action input must not be asked for _reasoning: %s", text(t, res))
	}
	if counter.store != 1 {
		t.Fatal("the action did not run")
	}
}

// TestActionSchemaOmitsReasoning: publishing it inside the action input would
// tell the model to send the field twice.
func TestActionSchemaOmitsReasoning(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Memory",
		Arguments: mustJSON(t, mcpserver.CompositeInput{
			Action: "store", Schema: true, Reasoning: "inspecting",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data mcpserver.ActionDetail `json:"data"`
	}
	if err := json.Unmarshal([]byte(text(t, res)), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, present := envelope.Data.InputSchema.Properties[command.ReasoningField]; present {
		t.Error("the action schema must not repeat _reasoning: it lives on the composite payload")
	}
	if _, present := envelope.Data.InputSchema.Properties["title"]; !present {
		t.Error("the action schema lost its own fields")
	}

	// The composite payload is where the model finds it.
	for _, tool := range listTools(t, session) {
		if tool.Name != "Memory" {
			continue
		}
		raw, _ := json.Marshal(tool.InputSchema)
		if !strings.Contains(string(raw), command.ReasoningField) {
			t.Errorf("the composite schema must declare %s: %s", command.ReasoningField, raw)
		}
	}
}

func TestCompositeRejectsAnUnknownAction(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Memory",
		Arguments: mustJSON(t, mcpserver.CompositeInput{
			Action: "obliterate", Reasoning: "testing",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("an unknown action must be an error")
	}
	body := text(t, res)
	if !strings.Contains(body, "AOS_COMMAND_ACTION_UNKNOWN") {
		t.Errorf("body = %s", body)
	}
	if !strings.Contains(body, "recall, store") {
		t.Errorf("the error must list the actions that do exist: %s", body)
	}
}

// TestCompositeRejectsAMissingReasoning reproduces the original's message.
func TestCompositeRejectsAMissingReasoning(t *testing.T) {
	reg, counter := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeComposite})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "Memory",
		Arguments: mustJSON(t, mcpserver.CompositeInput{Action: "store", Reasoning: "  "}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("an empty reasoning is a rejected call")
	}
	if counter.store != 0 {
		t.Fatal("the action ran despite the rejection")
	}
	if !strings.Contains(text(t, res), "An empty string is a rejected call") {
		t.Errorf("the message is not the original's: %s", text(t, res))
	}
}

// TestAToolErrorReachesTheModel: a protocol error ends the call, a tool error
// reaches the model — which can then read the code and the call to action.
func TestAToolErrorReachesTheModel(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "memories_store",
		Arguments: json.RawMessage(`{"_reasoning":"missing the title"}`),
	})
	if err != nil {
		t.Fatalf("a validation failure must not be a protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("the call should have failed")
	}
	body := text(t, res)
	if !strings.Contains(body, "AOS_COMMAND_VALIDATION_FAILED") {
		t.Errorf("body = %s", body)
	}
	if !strings.Contains(body, `"schema": true`) {
		t.Errorf("the error must carry the introspection path: %s", body)
	}
}

func TestBothShapesCoexist(t *testing.T) {
	reg, _ := newRegistry(t)
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeBoth})

	names := map[string]bool{}
	for _, tool := range listTools(t, session) {
		names[tool.Name] = true
	}
	for _, want := range []string{"memories_store", "memories_recall", "Memory", "Gateway"} {
		if !names[want] {
			t.Errorf("%s is missing from the both-shapes surface: %v", want, names)
		}
	}
}

// TestAnAliasKeepsWorkingAndTeachesTheNewName is the migration path the
// original never had: it renamed memories create/list/get/forgot in one release
// and broke every automation written against them.
func TestAnAliasKeepsWorkingAndTeachesTheNewName(t *testing.T) {
	reg, counter := newRegistry(t)
	reg.MustAlias(command.Alias{
		From: "memories_create", To: "memories_store",
		Since: "0.4.0", RemoveAt: "1.0.0", Deprecated: true,
	})
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})

	names := map[string]string{}
	for _, tool := range listTools(t, session) {
		names[tool.Name] = tool.Description
	}
	if _, ok := names["memories_create"]; !ok {
		t.Fatal("the old name must stay published")
	}
	if !strings.Contains(names["memories_create"], "DEPRECATED") {
		t.Errorf("the deprecation is not announced: %q", names["memories_create"])
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "memories_create",
		Arguments: mustJSON(t, storeInput{Title: "a memory", Reasoning: command.Reasoning{Reasoning: "why"}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("the alias must still work: %s", text(t, res))
	}
	if counter.store != 1 {
		t.Fatal("the alias did not route to the new command")
	}
	body := text(t, res)
	if !strings.Contains(body, `"_deprecated"`) || !strings.Contains(body, "memories_store") {
		t.Errorf("the result must teach the new name: %s", body)
	}
}

// TestThePrivilegeBoundaryIsNotAnMCPBoundary: gateway stays out of the agent's
// internal registry but remains available to a human's client, which is exactly
// how the original draws the line.
func TestThePrivilegeBoundaryIsNotAnMCPBoundary(t *testing.T) {
	reg, _ := newRegistry(t)
	for _, d := range reg.AgentTools() {
		if strings.HasPrefix(d.Key(), "gateway_") {
			t.Fatal("the agent must not reach the gateway")
		}
	}
	session := connect(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})
	found := false
	for _, tool := range listTools(t, session) {
		if tool.Name == "gateway_restart" {
			found = true
		}
	}
	if !found {
		t.Fatal("the command is still a published tool for a human's client")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/aos/internal/domain/workspace"
)

// /v/artifacts was mounted on the services of the workspace the daemon started
// in, while every command routes per workspace. An artifact created in any
// other workspace — listed, with a URL, by artifacts_list in that workspace —
// answered AOS_ARTIFACT_NOT_FOUND when the interface opened it.
func TestAnArtifactIsServedFromTheWorkspaceTheRequestNames(t *testing.T) {
	base, a := serving(t, nil)
	ctx := context.Background()
	if _, err := a.Config.Update(ctx, configOff()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Workspaces.Introspect(ctx, workspace.IntrospectInput{Path: a.Workspace}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Second", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}

	raw := invokeIn(inWorkspace("second"), t, a, "artifacts_create",
		`{"_reasoning":"a test is checking which workspace serves this","name":"Sales","visibility":"workspace"}`)
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		t.Fatalf("created = %s (%v)", raw, err)
	}

	get := func(ws string) int {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v/artifacts/"+created.ID+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		if ws != "" {
			req.Header.Set("X-Workspace-ID", ws)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	if got := get("second"); got != http.StatusOK {
		t.Errorf("the second workspace's artifact answered %d, want 200", got)
	}
	if got := get(""); got == http.StatusOK {
		t.Errorf("a request naming no workspace read the second workspace's artifact")
	}
}

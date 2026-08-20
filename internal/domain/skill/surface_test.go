package skill_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/skill"
)

// TestRegisterPublishesTheWholeGroup, and nothing else: which commands exist
// and their approval-risk annotations. What each one actually does is
// exercised below by invoking it, not by checking it is on this list — a
// command present here and never invoked proves only that Register ran.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	reg := command.NewRegistry()
	skill.Register(reg, inst)

	want := []string{
		"skills_create", "skills_delete", "skills_install",
		"skills_list", "skills_update",
	}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for _, d := range reg.Sorted() {
		switch d.Key() {
		case "skills_delete":
			if !d.Annotations().DestructiveHint {
				t.Error("uninstalling a skill must be announced destructive")
			}
		case "skills_install", "skills_create":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s writes to the workspace and must be announced destructive", d.Key())
			}
		}
		if !d.InRegistry() {
			t.Errorf("%s is not reachable by an agent", d.Key())
		}
	}
}

// TestListHandlerReturnsWhatIsInstalled drives skills_list through the same
// decode-validate-invoke path every surface uses, rather than calling
// Installer.List directly — the handler itself is what commands.go adds
// beyond List, and a call that never reaches it proves nothing about the
// wiring the registry actually runs.
func TestListHandlerReturnsWhatIsInstalled(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	reg := command.NewRegistry()
	skill.Register(reg, inst)
	d, _, ok := reg.Lookup("skills_list")
	if !ok {
		t.Fatal("skills_list is not registered")
	}

	out, err := d.Invoke(ctx(), command.SurfaceCLI, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(skill.ListOutput)
	if !ok {
		t.Fatalf("output = %T, want skill.ListOutput", out)
	}
	if got.Total != 1 || len(got.Skills) != 1 || got.Skills[0].ID != installed.ID {
		t.Fatalf("list = %+v", got)
	}
}

// TestInstallAndCreateShareTheHandler proves both names reach the same
// verified, consented write — the shared closure's whole reason for
// existing (see InstallRequest's own doc) — by actually installing through
// each and confirming the result is persisted, not merely returned.
func TestInstallAndCreateShareTheHandler(t *testing.T) {
	for _, name := range []string{"skills_install", "skills_create"} {
		t.Run(name, func(t *testing.T) {
			inst, _ := newInstaller(t, nil)
			reg := command.NewRegistry()
			skill.Register(reg, inst)
			d, _, ok := reg.Lookup(name)
			if !ok {
				t.Fatalf("%s is not registered", name)
			}

			raw, err := json.Marshal(skill.InstallRequest{Source: crmSource})
			if err != nil {
				t.Fatal(err)
			}
			out, err := d.Invoke(ctx(), command.SurfaceCLI, raw)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got, ok := out.(*skill.Skill)
			if !ok || got == nil {
				t.Fatalf("output = %T, want *skill.Skill", out)
			}
			if got.ID == "" || !got.Active {
				t.Fatalf("installed = %+v", got)
			}
			if _, err := inst.Get(ctx(), got.ID); err != nil {
				t.Fatalf("%s reported success but nothing was persisted: %v", name, err)
			}
		})
	}
}

// TestDeleteHandlerTrimsIDAndReportsWhatWasRemoved covers the one piece of
// logic deleteHandler adds over Uninstall itself: trimming the id it echoes
// back, and turning a bare error into the confirmation a caller reads.
func TestDeleteHandlerTrimsIDAndReportsWhatWasRemoved(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	reg := command.NewRegistry()
	skill.Register(reg, inst)
	d, _, ok := reg.Lookup("skills_delete")
	if !ok {
		t.Fatal("skills_delete is not registered")
	}

	raw, err := json.Marshal(skill.DeleteInput{ID: "  " + installed.ID + "  "})
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Invoke(ctx(), command.SurfaceCLI, raw)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(skill.DeleteOutput)
	if !ok || got.ID != installed.ID {
		t.Fatalf("output = %+v (%T), want DeleteOutput{ID: %q} — the padding should have been trimmed", out, out, installed.ID)
	}
	if _, err := inst.Get(ctx(), installed.ID); err == nil {
		t.Fatal("the skill is still there after skills_delete")
	}
}

// TestDeleteHandlerPropagatesAnUninstallFailure: a padded id that does not
// resolve still fails, rather than the trim silently producing a call that
// looks like it succeeded.
func TestDeleteHandlerPropagatesAnUninstallFailure(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	reg := command.NewRegistry()
	skill.Register(reg, inst)
	d, _, ok := reg.Lookup("skills_delete")
	if !ok {
		t.Fatal("skills_delete is not registered")
	}

	raw, err := json.Marshal(skill.DeleteInput{ID: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Invoke(ctx(), command.SurfaceCLI, raw); err == nil {
		t.Fatal("deleting an unknown skill through skills_delete reported success")
	}
}

// ---------------------------------------------------------------------------
// Update — skills_update's handler is a direct reference to Installer.Update
// (no closure of its own in commands.go), so it is exercised the same way
// every other domain in this codebase exercises a direct-reference handler:
// by calling the method itself.
// ---------------------------------------------------------------------------

func TestUpdateTurnsActiveOnAndOff(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)
	if !installed.Active {
		t.Fatal("fixture setup: expected a freshly installed skill to start active")
	}

	off := false
	got, err := inst.Update(ctx(), skill.UpdateInput{ID: installed.ID, Active: &off})
	if err != nil {
		t.Fatal(err)
	}
	if got.Active {
		t.Fatal("Update did not turn the skill off")
	}

	on := true
	got, err = inst.Update(ctx(), skill.UpdateInput{ID: installed.ID, Active: &on})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Active {
		t.Fatal("Update did not turn the skill back on")
	}

	again, err := inst.Get(ctx(), installed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Active {
		t.Fatal("the toggle was returned but not written back")
	}
}

// TestUpdateWithNoActiveLeavesItUnchanged: Active is the only field Update
// touches, and omitting it must mean "leave as is", not "turn off".
func TestUpdateWithNoActiveLeavesItUnchanged(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	got, err := inst.Update(ctx(), skill.UpdateInput{ID: installed.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Active {
		t.Fatal("Update with Active left nil turned the skill off")
	}
}

func TestUpdateOfAnUnknownSkillIsRefused(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	_, err := inst.Update(ctx(), skill.UpdateInput{ID: "nope"})
	if code := codeOf(t, err); code != "AOS_SKILL_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

// TestUpdateFailureWritingIsWrapped covers errUpdateFailed: Get succeeds,
// the repository write does not.
func TestUpdateFailureWritingIsWrapped(t *testing.T) {
	inst, _ := newInstaller(t, nil, withRepo(&updateFailingRepo{
		inner: fakes.NewRepo[skill.Skill]("skills"),
		err:   errors.New("no space left"),
	}))
	installed := mustInstall(t, inst)

	off := false
	_, err := inst.Update(ctx(), skill.UpdateInput{ID: installed.ID, Active: &off})
	if code := codeOf(t, err); code != "AOS_SKILL_UPDATE_FAILED" {
		t.Fatalf("code = %q", code)
	}
}

// updateFailingRepo makes Update fail while Get, List, Create and Delete are
// the real fakes.Repo underneath — the same shape as this file's own
// failingRepo, which fails Create instead, for the equivalent case in
// Install.
type updateFailingRepo struct {
	inner *fakes.Repo[skill.Skill]
	err   error
}

func (r *updateFailingRepo) Get(ctx context.Context, key collections.Key) (*skill.Skill, error) {
	return r.inner.Get(ctx, key)
}

func (r *updateFailingRepo) List(ctx context.Context, q collections.Query) ([]skill.Skill, error) {
	return r.inner.List(ctx, q)
}

func (r *updateFailingRepo) Create(ctx context.Context, v *skill.Skill) error {
	return r.inner.Create(ctx, v)
}

func (r *updateFailingRepo) Update(context.Context, *skill.Skill, collections.Version) error {
	return r.err
}

func (r *updateFailingRepo) Delete(ctx context.Context, key collections.Key) error {
	return r.inner.Delete(ctx, key)
}

// TestListPropagatesARepositoryFailure covers List's other branch: counting
// an unreadable skills directory as empty is how a review guard fails open.
func TestListPropagatesARepositoryFailure(t *testing.T) {
	inst, _ := newInstaller(t, nil, withRepo(listErrRepo{err: errors.New("no such directory")}))
	if _, err := inst.List(ctx()); err == nil {
		t.Fatal("an unreadable skills directory listed as empty")
	}
}

// listErrRepo fails every call. Only List is exercised above; the rest exist
// so it satisfies skill.Repository.
type listErrRepo struct{ err error }

func (r listErrRepo) Get(context.Context, collections.Key) (*skill.Skill, error) {
	return nil, r.err
}
func (r listErrRepo) List(context.Context, collections.Query) ([]skill.Skill, error) {
	return nil, r.err
}
func (r listErrRepo) Create(context.Context, *skill.Skill) error { return r.err }
func (r listErrRepo) Update(context.Context, *skill.Skill, collections.Version) error {
	return r.err
}
func (r listErrRepo) Delete(context.Context, collections.Key) error { return r.err }

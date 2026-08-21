package marketplace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/skill"
)

type fakeRegistry struct {
	listings  []marketplace.Listing
	searchErr error
	fetchPkg  skill.Package
	fetchErr  error
}

func (f *fakeRegistry) Search(context.Context, marketplace.SearchQuery) ([]marketplace.Listing, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.listings, nil
}

func (f *fakeRegistry) Fetch(context.Context, string, string) (skill.Package, error) {
	if f.fetchErr != nil {
		return skill.Package{}, f.fetchErr
	}
	return f.fetchPkg, nil
}

type fakeInstaller struct {
	gotSource string
	gotPkg    skill.Package
	result    *skill.Skill
	err       error
}

func (f *fakeInstaller) InstallPackage(_ context.Context, source string, pkg skill.Package, _ func(skill.Permissions) bool) (*skill.Skill, error) {
	f.gotSource = source
	f.gotPkg = pkg
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func reasoning() command.Reasoning { return command.Reasoning{Reasoning: "test"} }

func TestDiscoveryMergesEveryConfiguredRegistry(t *testing.T) {
	a := &fakeRegistry{listings: []marketplace.Listing{{Source: "acme/crm"}}}
	b := &fakeRegistry{listings: []marketplace.Listing{{Source: "acme/erp"}}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": a, "b": b},
		Order:      []string{"a", "b"},
		Installer:  &fakeInstaller{},
	})

	got, err := svc.Discovery(context.Background(), marketplace.DiscoveryInput{Reasoning: reasoning()})
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Discovery = %d listings, want 2", len(got))
	}
	if got[0].Registry != "a" || got[1].Registry != "b" {
		t.Fatalf("Discovery = %+v, want each listing tagged with the registry it came from", got)
	}
}

func TestDiscoverySkipsAFailingRegistryButReturnsWhatOthersFound(t *testing.T) {
	bad := &fakeRegistry{searchErr: errors.New("boom")}
	good := &fakeRegistry{listings: []marketplace.Listing{{Source: "acme/crm"}}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"bad": bad, "good": good},
		Order:      []string{"bad", "good"},
		Installer:  &fakeInstaller{},
	})

	got, err := svc.Discovery(context.Background(), marketplace.DiscoveryInput{Reasoning: reasoning()})
	if err != nil {
		t.Fatalf("Discovery: %v, want it to degrade past the one failing registry", err)
	}
	if len(got) != 1 || got[0].Source != "acme/crm" {
		t.Fatalf("Discovery = %+v, want just the reachable registry's listing", got)
	}
}

func TestDiscoveryFailsOnlyWhenEveryRegistryFails(t *testing.T) {
	a := &fakeRegistry{searchErr: errors.New("a is down")}
	b := &fakeRegistry{searchErr: errors.New("b is down")}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": a, "b": b},
		Order:      []string{"a", "b"},
		Installer:  &fakeInstaller{},
	})

	if _, err := svc.Discovery(context.Background(), marketplace.DiscoveryInput{Reasoning: reasoning()}); err == nil {
		t.Fatal("Discovery with every registry down succeeded, want a clear error")
	}
}

func TestDiscoveryWithNoRegistriesConfiguredIsRefused(t *testing.T) {
	svc := marketplace.NewService(marketplace.Deps{Installer: &fakeInstaller{}})
	if _, err := svc.Discovery(context.Background(), marketplace.DiscoveryInput{Reasoning: reasoning()}); err == nil {
		t.Fatal("Discovery with nothing configured succeeded, want a clear error naming the missing config")
	}
}

func TestInstallFetchesFromTheNamedRegistryAndInstallsThePackage(t *testing.T) {
	pkg := skill.Package{Manifest: skill.Manifest{Name: "crm"}}
	a := &fakeRegistry{fetchPkg: pkg}
	inst := &fakeInstaller{result: &skill.Skill{ID: "crm"}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": a},
		Order:      []string{"a"},
		Installer:  inst,
	})

	got, err := svc.Install(context.Background(), marketplace.InstallInput{Reasoning: reasoning(), Registry: "a", Source: "acme/crm"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got.ID != "crm" {
		t.Fatalf("Install returned %+v, want the installer's own result", got)
	}
	if inst.gotSource != "acme/crm" || inst.gotPkg.Manifest.Name != "crm" {
		t.Fatalf("installer got source=%q pkg=%+v, want the fetched package handed through unchanged", inst.gotSource, inst.gotPkg)
	}
}

func TestInstallTriesEveryRegistryUntilOneAnswers(t *testing.T) {
	down := &fakeRegistry{fetchErr: errors.New("down")}
	up := &fakeRegistry{fetchPkg: skill.Package{Manifest: skill.Manifest{Name: "crm"}}}
	inst := &fakeInstaller{result: &skill.Skill{ID: "crm"}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"down": down, "up": up},
		Order:      []string{"down", "up"},
		Installer:  inst,
	})

	if _, err := svc.Install(context.Background(), marketplace.InstallInput{Reasoning: reasoning(), Source: "acme/crm"}); err != nil {
		t.Fatalf("Install: %v, want it to fall through to the reachable registry", err)
	}
}

func TestInstallWithNoSourceIsRefused(t *testing.T) {
	svc := marketplace.NewService(marketplace.Deps{Installer: &fakeInstaller{}})
	if _, err := svc.Install(context.Background(), marketplace.InstallInput{Reasoning: reasoning()}); err == nil {
		t.Fatal("Install with no source succeeded, want a clear error")
	}
}

func TestInstallWithAnUnknownRegistryIsRefused(t *testing.T) {
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": &fakeRegistry{}},
		Order:      []string{"a"},
		Installer:  &fakeInstaller{},
	})
	if _, err := svc.Install(context.Background(), marketplace.InstallInput{Reasoning: reasoning(), Registry: "nope", Source: "acme/crm"}); err == nil {
		t.Fatal("Install naming an unconfigured registry succeeded, want a clear error")
	}
}

func TestGetFindsAListingBySource(t *testing.T) {
	a := &fakeRegistry{listings: []marketplace.Listing{{Source: "acme/crm", Name: "CRM"}}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": a},
		Order:      []string{"a"},
		Installer:  &fakeInstaller{},
	})

	got, err := svc.Get(context.Background(), marketplace.GetInput{Reasoning: reasoning(), Source: "acme/crm"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "CRM" || got.Registry != "a" {
		t.Fatalf("Get = %+v, want the matching listing tagged with its registry", got)
	}
}

func TestGetOfAnUnknownSourceIsRefused(t *testing.T) {
	a := &fakeRegistry{listings: []marketplace.Listing{{Source: "acme/crm"}}}
	svc := marketplace.NewService(marketplace.Deps{
		Registries: map[string]marketplace.Registry{"a": a},
		Order:      []string{"a"},
		Installer:  &fakeInstaller{},
	})

	if _, err := svc.Get(context.Background(), marketplace.GetInput{Reasoning: reasoning(), Source: "nobody/nothing"}); err == nil {
		t.Fatal("Get of an unlisted source succeeded, want a clear error")
	}
}

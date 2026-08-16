package theme

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed themes/*.yaml
var builtinFS embed.FS

// tokens.txt is the contract between a theme and the stylesheet: the custom
// properties the frontend actually reads. It is generated from the CSS rather
// than maintained here, so a property added to a component and forgotten in the
// renderer fails the build instead of rendering something invisible.
//
//go:embed tokens.txt
var requiredTokensRaw string

// Service is the theme aggregate.
type Service struct {
	builtin []Theme
	store   Store
	log     *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	// Store is optional. Without it the built-in themes still work and
	// installing one is refused with a reason, which is what a run with no
	// state directory should do.
	Store Store
	Log   *slog.Logger
}

// NewService loads the built-in themes and wires the service.
//
// A built-in theme that does not parse is a build error, not a runtime one:
// they are embedded, so if one is broken it was broken when the binary was
// made, and starting anyway would hide it.
func NewService(d Deps) (*Service, error) {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	loaded, err := loadBuiltin()
	if err != nil {
		return nil, err
	}
	return &Service{builtin: loaded, store: d.Store, log: log}, nil
}

func loadBuiltin() ([]Theme, error) {
	entries, err := builtinFS.ReadDir("themes")
	if err != nil {
		return nil, err
	}
	out := make([]Theme, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := builtinFS.ReadFile("themes/" + e.Name())
		if err != nil {
			return nil, err
		}
		var t Theme
		if err := yaml.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("theme %s: %w", e.Name(), err)
		}
		t.ID = normalizeID(t.ID)
		t.Builtin = true
		if err := Validate(t); err != nil {
			return nil, fmt.Errorf("theme %s: %w", e.Name(), err)
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// RequiredTokens is the contract every rendered theme must satisfy.
func RequiredTokens() []string {
	var out []string
	for _, line := range strings.Split(requiredTokensRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.TrimPrefix(line, "--"))
	}
	sort.Strings(out)
	return out
}

// Validate checks that a theme can be rendered and that the rendering defines
// every token the stylesheet reads.
//
// It renders rather than inspecting fields, because a theme is four colours and
// the tokens are derived: the only honest way to know a theme produces what the
// interface needs is to produce it.
func Validate(t Theme) error {
	if normalizeID(t.ID) == "" {
		return errInvalidID(t.ID)
	}
	if len(t.Variants) == 0 {
		return errNoVariants(t.ID)
	}
	for appearance := range t.Variants {
		if appearance != Light && appearance != Dark {
			return errUnknownVariant(t.ID, string(appearance))
		}
	}

	required := RequiredTokens()
	for _, appearance := range t.Appearances() {
		rendered, err := Render(t, appearance, true)
		if err != nil {
			return err
		}
		var missing []string
		for _, name := range required {
			if _, ok := rendered.Tokens[name]; !ok {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return errMissingTokens(t.ID, string(appearance), missing)
		}
	}
	return nil
}

// List returns the built-in themes and the user's, built-in first.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	out := ListOutput{Themes: append([]Theme(nil), s.builtin...)}
	if s.store != nil {
		installed, err := s.store.List(ctx)
		if err != nil {
			return ListOutput{}, errReadFailed("List", err)
		}
		sort.Slice(installed, func(i, j int) bool { return installed[i].ID < installed[j].ID })
		out.Themes = append(out.Themes, installed...)
	}
	if in.Appearance != "" {
		if !in.Appearance.Valid() {
			return ListOutput{}, errInvalidAppearance(string(in.Appearance))
		}
		filtered := out.Themes[:0]
		for _, t := range out.Themes {
			// Asking for auto means asking for the themes that can follow the
			// system, which is the ones carrying both palettes.
			if _, ok := t.Variants[in.Appearance]; ok || (in.Appearance == Auto && t.Appearance() == Auto) {
				filtered = append(filtered, t)
			}
		}
		out.Themes = filtered
	}
	out.Total = len(out.Themes)
	return out, nil
}

// Get reads one theme, rendered for an appearance.
func (s *Service) Get(ctx context.Context, in GetInput) (GetOutput, error) {
	found, err := s.find(ctx, in.ID)
	if err != nil {
		return GetOutput{}, err
	}
	appearance := in.Appearance
	if appearance == "" {
		appearance = Auto
	}
	if !appearance.Valid() {
		return GetOutput{}, errInvalidAppearance(string(appearance))
	}

	out := GetOutput{Theme: *found}
	for _, a := range found.Appearances() {
		rendered, err := Render(*found, a, in.Native)
		if err != nil {
			return GetOutput{}, err
		}
		out.Rendered = append(out.Rendered, rendered)
	}
	return out, nil
}

// Install adds a user preset, validating it first.
func (s *Service) Install(ctx context.Context, in InstallInput) (*Theme, error) {
	if s.store == nil {
		return nil, errStoreUnavailable()
	}
	t := Theme{
		ID: normalizeID(in.ID), Name: strings.TrimSpace(in.Name),
		Author: in.Author, Variants: in.Variants,
	}
	if t.Name == "" {
		t.Name = t.ID
	}
	for _, builtin := range s.builtin {
		if builtin.ID == t.ID {
			return nil, errShadowsBuiltin(t.ID)
		}
	}
	if err := Validate(t); err != nil {
		return nil, err
	}
	if err := s.store.Save(ctx, t); err != nil {
		return nil, errWriteFailed("Install", err)
	}
	return &t, nil
}

// Delete removes a user preset. A built-in one cannot be deleted: it is in the
// binary, so removing it would only mean pretending it is not there.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := normalizeID(in.ID)
	for _, builtin := range s.builtin {
		if builtin.ID == id {
			return DeleteOutput{}, errBuiltinIsPermanent(id)
		}
	}
	if s.store == nil {
		return DeleteOutput{}, errStoreUnavailable()
	}
	if _, err := s.store.Get(ctx, id); err != nil {
		return DeleteOutput{}, errNotFound(id)
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}

func (s *Service) find(ctx context.Context, id string) (*Theme, error) {
	wanted := normalizeID(id)
	for i := range s.builtin {
		if s.builtin[i].ID == wanted {
			return &s.builtin[i], nil
		}
	}
	if s.store != nil {
		if found, err := s.store.Get(ctx, wanted); err == nil && found != nil {
			return found, nil
		}
	}
	return nil, errNotFound(wanted)
}

package theme

import "github.com/OWNER/aos/internal/core/command"

// ListInput selects themes.
type ListInput struct {
	Appearance Appearance `json:"appearance,omitempty" cli:"flag" jsonschema:"Only themes that offer this appearance: light, dark or auto."`

	command.Reasoning
}

// ListOutput is the presets available.
type ListOutput struct {
	Themes []Theme `json:"themes" jsonschema:"The themes, built-in first."`
	Total  int     `json:"total" jsonschema:"How many there are."`
}

// GetInput names one theme.
type GetInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the theme." validate:"required,notblank"`

	Appearance Appearance `json:"appearance,omitempty" cli:"flag" jsonschema:"Which palette to render. Defaults to both."`

	// Native reports whether the caller is the desktop. A blur window means
	// nothing in a browser: there is nothing behind the page to blur, and a
	// translucent background there just shows white.
	Native bool `json:"native,omitempty" cli:"flag" jsonschema:"True when rendering for the desktop window, where a translucent background has something behind it."`

	command.Reasoning
}

// GetOutput carries the theme and its rendered tokens.
type GetOutput struct {
	Theme    Theme      `json:"theme" jsonschema:"The theme as its author wrote it."`
	Rendered []Rendered `json:"rendered" jsonschema:"The CSS custom properties for each appearance it offers."`
}

// InstallInput adds a user preset.
type InstallInput struct {
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier for the theme. Example: \"my-theme\"." validate:"required,notblank"`
	Name string `json:"name,omitempty" cli:"flag" jsonschema:"Human-readable name. Defaults to the identifier."`

	Author   string                 `json:"author,omitempty" cli:"flag" jsonschema:"Who made it."`
	Variants map[Appearance]Palette `json:"variants" jsonschema:"Palettes by appearance. A theme with both light and dark can follow the system." validate:"required"`

	command.Reasoning
}

// DeleteInput removes a user preset.
type DeleteInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the theme." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput names what went.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"The theme that was removed."`
}

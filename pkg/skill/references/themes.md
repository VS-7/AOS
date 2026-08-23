# Themes

The appearance of the application.

The colour presets the interface can wear.

A theme is four colours and a dial — a surface, an ink, an accent, and how
strongly the layers between them separate. Everything else is derived: the
cards, the borders, the chart series, and the colour of every memory category
in the graph.

## Commands
- **list** — the themes available, built-in first
- **get** — one theme with the CSS properties it renders to
- **install** — add a preset of your own
- **delete** — remove one of yours

## When to use
- **Someone asks to change how the app looks:** list, then set it in config
- **Someone brings a palette:** install it as a preset

## When NOT to use
- Not to change one colour in one screen — that is a component, not a theme

## Commands

### `themes_delete`

Remove a theme you installed.

Delete one of your presets.

A built-in theme cannot be removed: it lives in the binary, so deleting it would
only mean pretending it is not there.

- remove a preset

### `themes_get`

Read one theme and what it renders to.

One theme: the palette its author wrote, and the CSS custom properties
that palette produces for each appearance it offers.

The rendered properties are what the interface actually sets on the document
root. Reading them is how you find out what a theme will look like without
switching to it.

- read a theme
- as the desktop window would render it

### `themes_install`

Add a theme of your own.

Install a preset.

It is rendered and checked before it is stored: a palette that produces fewer
properties than the interface reads is refused here rather than discovered later
as an element nobody can see.

- a dark preset

### `themes_list`

List the themes available.

The presets this build ships with, followed by the ones installed here.

- everything available
- only what can go light


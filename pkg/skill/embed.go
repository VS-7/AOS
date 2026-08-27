package skill

import "embed"

// Name is the skill's own name: the frontmatter `name` of SKILL.md and the
// directory it is installed into (<skills dir>/aos/SKILL.md). Agent harnesses
// match the two, so they cannot drift apart here either.
const Name = "aos"

// Files is the published skill, compiled into every binary that imports this
// package. It is what lets `aos self skill install` and the desktop's own
// "Add skill to…" put the skill on a machine that has never seen this
// repository: the files an agent needs travel inside the executable rather
// than being fetched from somewhere that may be down, ahead, or gone.
//
// The pattern names SKILL.md and references/*.md explicitly. Anything else in
// this directory — the Go sources, the tests — is not part of the skill and
// must not be shipped as if it were.
//
//go:embed SKILL.md references/*.md
var Files embed.FS

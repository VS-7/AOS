package slug_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/slug"
)

// TestGenerateMatchesTheOriginal pins the rule against the cases the original's
// helper documents, plus the ones that decide whether two names collide.
func TestGenerateMatchesTheOriginal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World!", "hello-world"},
		{"Café au Lait", "cafe-au-lait"},
		{"Project Alpha", "project-alpha"},
		{"Atlas", "atlas"},
		{"  spaced  out  ", "spaced-out"},
		{"multi---hyphen", "multi-hyphen"},
		{"-leading and trailing-", "leading-and-trailing"},
		{"snake_case_kept", "snake_case_kept"},
		{"MiXeD CaSe", "mixed-case"},
		{"Ação & Reação", "acao-reacao"},
		{"v0.1.401", "v01401"},
		{"", ""},
		{"   ", ""},
		{"!!!", ""},
		{"---", ""},
	}
	for _, c := range cases {
		if got := slug.Generate(c.in); got != c.want {
			t.Errorf("Generate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGenerateIsIdempotent is what makes the slug usable as an identity: the
// slug of a slug is the same slug, so normalising twice cannot move a record.
func TestGenerateIsIdempotent(t *testing.T) {
	for _, in := range []string{
		"Hello World!", "Café au Lait", "  --  x  --  ", "já-normalizado", "a_b-c1",
	} {
		once := slug.Generate(in)
		if twice := slug.Generate(once); twice != once {
			t.Errorf("Generate(%q) = %q, but Generate(%q) = %q", in, once, once, twice)
		}
	}
}

// TestDistinctNamesThatCollide records a real consequence rather than a bug: two
// names can produce one slug, so the aggregate that uses it must reject the
// duplicate rather than assume uniqueness.
func TestDistinctNamesThatCollide(t *testing.T) {
	if a, b := slug.Generate("Project Alpha"), slug.Generate("project-alpha!"); a != b {
		t.Fatalf("expected a collision, got %q and %q", a, b)
	}
}

func TestIsValid(t *testing.T) {
	for _, s := range []string{"atlas", "project-alpha", "snake_case", "a1"} {
		if !slug.IsValid(s) {
			t.Errorf("IsValid(%q) = false", s)
		}
	}
	for _, s := range []string{"", "Atlas", "with space", "trailing-", "-leading", "dot.ted"} {
		if slug.IsValid(s) {
			t.Errorf("IsValid(%q) = true", s)
		}
	}
}

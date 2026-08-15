package search_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/aos/internal/core/search"
)

func TestTokenizeDropsStopWordsAndPunctuation(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"UUID migration", []string{"uuid", "migration"}},
		{"the gateway is on", []string{"gateway"}},
		{"Felipe's commit-message preference", []string{"felipe", "s", "commit", "message", "preference"}},
		{"v0.1.401", []string{"v0", "1", "401"}},
		{"", nil},
		{"   ", nil},
		{"the of and", nil},
	}
	for _, c := range cases {
		got := search.Tokenize(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestAQueryOfOnlyStopWordsMatchesEverything records the rule: it tokenises to
// nothing, and a query with no terms is not a filter.
func TestAQueryOfOnlyStopWordsMatchesEverything(t *testing.T) {
	d := search.Document{Text: map[string]string{"title": "anything"}}
	if !search.Matches(d, search.Tokenize("the and of")) {
		t.Fatal("a query with no meaningful terms must not exclude anything")
	}
}

func doc(title, description, content, tags string) search.Document {
	return search.Document{Text: map[string]string{
		"title": title, "description": description, "content": content, "tags": tags,
	}}
}

// TestMatchesRequiresEveryToken: a recall that returns records containing only
// one of three words is worse than one that returns nothing, because the caller
// acts on what comes back.
func TestMatchesRequiresEveryToken(t *testing.T) {
	d := doc("UUID migration decision", "Chose UUID v4 over auto-increment", "", "database")

	if !search.Matches(d, search.Tokenize("uuid migration")) {
		t.Error("both tokens are present and it did not match")
	}
	if search.Matches(d, search.Tokenize("uuid postgres")) {
		t.Error("one token is absent and it matched anyway")
	}
}

func TestMatchesIsCaseInsensitiveAndSpansFields(t *testing.T) {
	d := doc("UUID migration", "", "The gateway restarts nightly", "Database")
	if !search.Matches(d, search.Tokenize("uuid GATEWAY database")) {
		t.Fatal("a query spanning title, content and tags did not match")
	}
}

// TestScoreRanksTheTitleAboveTheBody is the judgement the weights encode: the
// body is long enough that any word eventually turns up in it.
func TestScoreRanksTheTitleAboveTheBody(t *testing.T) {
	inTitle := doc("gateway", "", "unrelated prose", "")
	inBody := doc("unrelated", "", "gateway", "")

	tokens := search.Tokenize("gateway")
	if search.Score(inTitle, tokens) <= search.Score(inBody, tokens) {
		t.Fatalf("title %.1f, body %.1f", search.Score(inTitle, tokens), search.Score(inBody, tokens))
	}
}

func TestScoreCountsRepeats(t *testing.T) {
	once := doc("", "", "gateway", "")
	twice := doc("", "", "gateway gateway", "")
	tokens := search.Tokenize("gateway")
	if search.Score(twice, tokens) <= search.Score(once, tokens) {
		t.Fatal("a document mentioning the term twice should rank above one mentioning it once")
	}
}

func TestScoreOfNoMatchIsZero(t *testing.T) {
	if got := search.Score(doc("a", "b", "c", "d"), search.Tokenize("absent")); got != 0 {
		t.Fatalf("score = %v", got)
	}
}

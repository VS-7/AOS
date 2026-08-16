package routine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

// TestRegisterPublishesTheWholeGroup.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	h := newHarness(t)
	reg := command.NewRegistry()
	Register(reg, h.svc)

	want := []string{
		"routines_create", "routines_delete", "routines_fire", "routines_get",
		"routines_list", "routines_rotate", "routines_runs", "routines_update",
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
		case "routines_list", "routines_get", "routines_runs":
			if !d.Annotations().ReadOnlyHint {
				t.Errorf("%s must be announced read-only", d.Key())
			}
		case "routines_delete":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s must be announced destructive", d.Key())
			}
		}
	}
}

// TestListReportsTheSchedulerTickAlongsideTheRoutines, because the cron a user
// wrote and the resolution they get are different numbers.
func TestListReportsTheSchedulerTickAlongsideTheRoutines(t *testing.T) {
	h := newHarness(t)
	h.create(t, CreateInput{Name: "On", Triggers: []TriggerInput{{Type: Scheduled, Cron: "0 9 * * *"}}})
	h.create(t, CreateInput{Name: "Off", Status: Disabled})

	all, err := h.svc.List(asAgent("atlas"), ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 {
		t.Fatalf("total = %d", all.Total)
	}
	if all.Tick != "15m0s" {
		t.Fatalf("tick = %q", all.Tick)
	}

	enabled, err := h.svc.List(asAgent("atlas"), ListInput{Status: Enabled})
	if err != nil {
		t.Fatal(err)
	}
	if enabled.Total != 1 || enabled.Routines[0].Name != "On" {
		t.Fatalf("enabled = %+v", enabled.Routines)
	}
	if enabled.Routines[0].NextRun == nil {
		t.Fatal("a scheduled routine does not say when it fires next")
	}
}

// TestUpdateChangesEveryFieldItAdvertises, and replacing the triggers with a
// webhook mints a new token.
func TestUpdateChangesEveryFieldItAdvertises(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Original"})

	changed, err := h.svc.Update(asAgent("atlas"), UpdateInput{
		ID:       out.Routine.ID,
		Name:     ptr("Renamed"),
		Status:   ptr(Disabled),
		Scope:    &Scope{AllowExternalCalls: true},
		Content:  ptr("\n\nDo the thing."),
		Triggers: &[]TriggerInput{{Type: Webhook}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Routine.Name != "Renamed" || changed.Routine.Status != Disabled {
		t.Fatalf("got %+v", changed.Routine.Routine)
	}
	if !changed.Routine.Scope.AllowExternalCalls {
		t.Fatal("the scope did not change")
	}
	if changed.Routine.Content != "Do the thing." {
		t.Fatalf("content = %q — the leading blank lines were not trimmed", changed.Routine.Content)
	}
	if changed.Token == "" {
		t.Fatal("adding a webhook trigger minted no token")
	}
}

// TestTheInputsThatCannotBeAccepted.
func TestTheInputsThatCannotBeAccepted(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Plain"})

	cases := []struct {
		name string
		call func() error
		code string
	}{
		{"a name with nothing in it", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{Name: "  "})
			return err
		}, "ROUTINE_INVALID_NAME"},
		{"a status that is not one", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{Name: "x", Status: "paused"})
			return err
		}, "ROUTINE_INVALID_STATUS"},
		{"no agent at all", func() error {
			_, err := h.svc.Create(context.Background(), CreateInput{Name: "x"})
			return err
		}, "ROUTINE_AGENT_REQUIRED"},
		{"an agent that is not there", func() error {
			_, err := h.svc.Create(asAgent("ghost"), CreateInput{Name: "x"})
			return err
		}, "ROUTINE_NO_SUCH_AGENT"},
		{"a routine that is not there", func() error {
			_, err := h.svc.Get(asAgent("atlas"), GetInput{ID: "r-missing"})
			return err
		}, "ROUTINE_NOT_FOUND"},
		{"a cron that does not parse", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{Type: Scheduled, Cron: "every friday"}},
			})
			return err
		}, "ROUTINE_INVALID_CRON"},
		{"filters on a scheduled trigger", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{
					Type: Scheduled, Cron: "0 9 * * *",
					Filters: []Filter{{Field: "type", Operator: OpEq, Value: "bug"}},
				}},
			})
			return err
		}, "ROUTINE_FILTERS_NOT_APPLICABLE"},
		{"filters on a webhook trigger", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{
					Type: Webhook, Filters: []Filter{{Field: "type", Operator: OpEq, Value: "bug"}},
				}},
			})
			return err
		}, "ROUTINE_FILTERS_NOT_APPLICABLE"},
		{"an activity trigger with no namespace", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{Type: Activity}},
			})
			return err
		}, "ROUTINE_ACTIVITY_NAMESPACE_REQUIRED"},
		{"a filter operator that is not one", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{
					Type: Activity, Namespace: "task",
					Filters: []Filter{{Field: "type", Operator: "startsWith", Value: "b"}},
				}},
			})
			return err
		}, "ROUTINE_UNKNOWN_OPERATOR"},
		{"a filter with no field", func() error {
			_, err := h.svc.Create(asAgent("atlas"), CreateInput{
				Name: "x", Triggers: []TriggerInput{{
					Type: Activity, Namespace: "task",
					Filters: []Filter{{Operator: OpEq, Value: "b"}},
				}},
			})
			return err
		}, "ROUTINE_FILTER_FIELD_REQUIRED"},
		{"rotating a routine with no webhook", func() error {
			_, err := h.svc.Rotate(asAgent("atlas"), RotateInput{ID: out.Routine.ID})
			return err
		}, "ROUTINE_NO_WEBHOOK_TRIGGER"},
		{"an empty name on update", func() error {
			_, err := h.svc.Update(asAgent("atlas"), UpdateInput{ID: out.Routine.ID, Name: ptr(" ")})
			return err
		}, "ROUTINE_INVALID_NAME"},
		{"a status that is not one, on update", func() error {
			_, err := h.svc.Update(asAgent("atlas"), UpdateInput{
				ID: out.Routine.ID, Status: ptr(Status("paused")),
			})
			return err
		}, "ROUTINE_INVALID_STATUS"},
	}
	for _, tc := range cases {
		err := tc.call()
		if err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
		got, ok := apperr.As(err)
		if !ok || !strings.HasSuffix(got.Code, tc.code) {
			t.Fatalf("%s: error = %v, want %s", tc.name, err, tc.code)
		}
	}
}

// TestWithoutARuntimeFiringSaysSoAndRecordsTheAttempt.
func TestWithoutARuntimeFiringSaysSoAndRecordsTheAttempt(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Executor = nil })
	out := h.create(t, CreateInput{Name: "Nowhere to run"})

	_, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID})
	if err == nil {
		t.Fatal("firing succeeded with no runtime")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "ROUTINE_NO_EXECUTOR") {
		t.Fatalf("error = %v", err)
	}
	history, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Runs) != 1 || history.Runs[0].Status != RunFailed {
		t.Fatalf("runs = %+v", history.Runs)
	}
}

// TestWithoutATokenMinterAWebhookTriggerIsRefusedRatherThanStoredOpen.
func TestWithoutATokenMinterAWebhookTriggerIsRefusedRatherThanStoredOpen(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Tokens = nil })

	_, err := h.svc.Create(asAgent("atlas"), CreateInput{
		Name: "Hooked", Triggers: []TriggerInput{{Type: Webhook}},
	})
	if err == nil {
		t.Fatal("a webhook trigger was stored with no token at all")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "ROUTINE_TOKENS_UNAVAILABLE") {
		t.Fatalf("error = %v", err)
	}
}

// TestTheRunHistoryIsNewestFirstAndPageable.
func TestTheRunHistoryIsNewestFirstAndPageable(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Repeats"})
	for range 3 {
		if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err != nil {
			t.Fatal(err)
		}
	}

	history, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if history.Total != 3 || len(history.Runs) != 2 {
		t.Fatalf("total %d, page %d", history.Total, len(history.Runs))
	}
	if history.Runs[0].StartedAt.Before(history.Runs[1].StartedAt) {
		t.Fatal("the history is oldest first")
	}
}

// TestCronParsing walks the syntax the scheduler claims to understand, and the
// syntax it does not.
func TestCronParsing(t *testing.T) {
	valid := map[string]string{
		"* * * * *":    "every minute",
		"0 9 * * 1-5":  "weekday mornings",
		"*/15 * * * *": "every quarter hour",
		"0,30 * * * *": "on the hour and the half hour",
		"0 0 1 * *":    "the first of the month",
		"0 0 1-7 * 1":  "the first Monday, by cron's OR rule",
		"30 2 * 3,9 *": "March and September",
		"0 0 * * 0":    "Sundays",
		"0 */6 * * *":  "every six hours",
	}
	for expr, what := range valid {
		if _, err := Parse(expr); err != nil {
			t.Errorf("%s (%s): %v", expr, what, err)
		}
	}

	invalid := []string{
		"", "* * * *", "* * * * * *",
		"60 * * * *", "* 24 * * *", "* * 32 * *", "* * * 13 *", "* * * * 7",
		"every friday", "@daily", "*/0 * * * *", "5-1 * * * *", "a * * * *",
	}
	for _, expr := range invalid {
		if _, err := Parse(expr); err == nil {
			t.Errorf("%q parsed", expr)
		}
	}
}

// TestCronsOrRuleForDayAndWeekday. When both the day of month and the day of
// week are narrowed, cron ORs them — a scheduler that ANDs them silently never
// fires.
func TestCronsOrRuleForDayAndWeekday(t *testing.T) {
	s, err := Parse("0 0 13 * 5")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"2026-03-13T00:00:00Z": true,  // the 13th, and a Friday
		"2026-05-13T00:00:00Z": true,  // the 13th, a Wednesday
		"2026-03-06T00:00:00Z": true,  // a Friday, the 6th
		"2026-03-05T00:00:00Z": false, // neither
	}
	for stamp, want := range cases {
		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Matches(at); got != want {
			t.Errorf("%s matched %v, want %v", stamp, got, want)
		}
	}
}

// TestAWindowWithNoPreviousFiringLooksBackOneTick, so a routine created between
// ticks does not fire once for every minute since the epoch.
func TestAWindowWithNoPreviousFiringLooksBackOneTick(t *testing.T) {
	s, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatal(err)
	}
	nine := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

	if !DueInWindow(s, time.Time{}, nine, 15*time.Minute) {
		t.Fatal("a routine that has never fired missed the slot inside its first window")
	}
	if DueInWindow(s, time.Time{}, nine.Add(2*time.Hour), 15*time.Minute) {
		t.Fatal("a window two hours later still fired the nine-o'clock slot")
	}
	if DueInWindow(nil, time.Time{}, nine, 0) {
		t.Fatal("a nil schedule fired")
	}
	if DueInWindow(s, nine, nine, 0) {
		t.Fatal("a window of zero width fired")
	}
}

// TestRenderNormalisesTheValuesAFilterCompares. A filter written as 1 has to
// match a payload carrying 1.0, because JSON has one number type and YAML has
// another.
func TestRenderNormalisesTheValuesAFilterCompares(t *testing.T) {
	cases := map[string]any{
		"":      nil,
		"bug":   "bug",
		"true":  true,
		"false": false,
		"3":     float64(3),
		"3.5":   3.5,
		"7":     int(7),
		"8":     int64(8),
		"9":     float32(9),
		"[1 2]": []int{1, 2},
	}
	for want, in := range cases {
		if got := render(in); got != want {
			t.Errorf("render(%v) = %q, want %q", in, got, want)
		}
	}
}

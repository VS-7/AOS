package task

// transitions is the authoritative lifecycle graph. Anything not listed is
// refused with AOS_TASK_INVALID_TRANSITION — a status is never a plain field
// write, so this table is the only description of how work moves.
//
// Two edges are worth naming. InReview can go back to InProgress, because a
// review that finds something is the point of having one. Finished has no
// outgoing edge at all: reopening finished work means creating the task that
// says what was wrong with it, which leaves a record that reopening does not.
var transitions = map[Status][]Status{
	Suggestion: {Backlog, Finished},
	Backlog:    {Planning, Todo, Stopped},
	Planning:   {Todo, Backlog, Stopped},
	Todo:       {InProgress, Backlog, Stopped},
	InProgress: {InReview, Stopped, Todo},
	Stopped:    {InProgress, Todo, Backlog},
	InReview:   {Finished, InProgress},
	Finished:   {},
}

// CanMoveTo reports whether the lifecycle allows this move.
func (s Status) CanMoveTo(next Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// NextStates lists where this status can go, for the error that says what was
// possible instead of only what was not.
func (s Status) NextStates() []string {
	out := make([]string, 0, len(transitions[s]))
	for _, next := range transitions[s] {
		out = append(out, string(next))
	}
	return out
}

// Terminal reports whether the task is done and will not move again.
func (s Status) Terminal() bool { return s == Finished }

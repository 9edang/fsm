package fsm

import "context"

// GuardFunc is a pre-condition that must return nil for a transition to proceed.
// If it returns an error the transition is rejected and state is not changed.
type GuardFunc func(ctx context.Context) error

// ActionFunc is a side effect executed after a transition succeeds.
// When an action returns an error the state has already changed — callers must
// handle compensation themselves if needed.
type ActionFunc func(ctx context.Context) error

// transitionDef is the internal, immutable representation of one transition rule.
type transitionDef struct {
	event   Event
	from    []State
	to      State
	guards  []GuardFunc
	actions []ActionFunc
}

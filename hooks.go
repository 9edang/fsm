package fsm

import "context"

// TransitionHookFunc is a callback invoked around every transition.
// It receives the context plus the from/to states and triggering event.
type TransitionHookFunc func(ctx context.Context, from, to State, event Event)

// StateHookFunc is a callback invoked when a specific state is entered or exited.
type StateHookFunc func(ctx context.Context)

// ErrorHookFunc is a callback invoked when a guard or action returns an error.
type ErrorHookFunc func(ctx context.Context, err error)

// hooks holds all registered global and per-state callbacks.
// The zero value is valid and means no hooks are active.
type hooks struct {
	before []TransitionHookFunc
	after  []TransitionHookFunc
	onErr  []ErrorHookFunc
	enter  map[State][]StateHookFunc // state → fns called on entry
	exit   map[State][]StateHookFunc // state → fns called on exit
}

func newHooks() *hooks {
	return &hooks{
		enter: make(map[State][]StateHookFunc),
		exit:  make(map[State][]StateHookFunc),
	}
}

func (h *hooks) runBefore(ctx context.Context, from, to State, event Event) {
	for _, fn := range h.before {
		fn(ctx, from, to, event)
	}
}

func (h *hooks) runAfter(ctx context.Context, from, to State, event Event) {
	for _, fn := range h.after {
		fn(ctx, from, to, event)
	}
}

func (h *hooks) runOnError(ctx context.Context, err error) {
	for _, fn := range h.onErr {
		fn(ctx, err)
	}
}

func (h *hooks) runExit(ctx context.Context, state State) {
	for _, fn := range h.exit[state] {
		fn(ctx)
	}
}

func (h *hooks) runEnter(ctx context.Context, state State) {
	for _, fn := range h.enter[state] {
		fn(ctx)
	}
}

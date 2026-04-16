package fsm

import (
	"context"
	"fmt"
	"sync"
)

// State is a named condition of an object managed by the FSM.
// It is a distinct named type — not an alias for string — so the compiler
// will reject a plain string variable where a State is expected.
// Untyped string literals and typed constants (const S State = "s") both work.
type State string

// Event is a named trigger that causes a state transition.
// Same type-safety rules as State apply.
type Event string

// ---------------------------------------------------------------------------
// Internal definition
// ---------------------------------------------------------------------------

// definition is the validated, immutable FSM schema produced by Build.
type definition struct {
	initial     State
	transitions []*transitionDef
	hooks       *hooks
	withHistory bool
	// index maps event → state → transition for O(1) lookup.
	// Built once during Build() and immutable thereafter.
	index map[Event]map[State]*transitionDef
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

// Builder constructs an FSM definition using a fluent, declarative API.
// Call New to start, chain On/From/To/Guard/Action calls, then Build.
type Builder struct {
	initial     State
	transitions []*transitionDef
	hooks       *hooks
	withHistory bool
}

// New creates a new Builder with the given initial state.
func New(initial State) *Builder {
	return &Builder{initial: initial, hooks: newHooks()}
}

// On registers a new transition for the given event.
// It returns a TransitionBuilder for further configuration.
func (b *Builder) On(event Event) *TransitionBuilder {
	def := &transitionDef{event: event}
	b.transitions = append(b.transitions, def)
	return &TransitionBuilder{b: b, def: def}
}

// Build validates the accumulated transition definitions and returns a ready FSM.
// It returns an error if the definition is incomplete or inconsistent.
func (b *Builder) Build() (*FSM, error) {
	if err := validateBuilder(b); err != nil {
		return nil, err
	}
	def := &definition{
		initial:     b.initial,
		transitions: b.transitions,
		hooks:       b.hooks,
		withHistory: b.withHistory,
		index:       buildTransitionIndex(b.transitions),
	}
	f := &FSM{current: def.initial, def: def}
	if def.withHistory {
		f.hist = &history{}
	}
	return f, nil
}

// validateBuilder checks the builder for completeness and consistency.
func validateBuilder(b *Builder) error {
	if b.initial == "" {
		return fmt.Errorf("fsm: initial state must not be empty")
	}
	// "event:from" pair must be unique across all transitions
	seen := make(map[string]struct{})
	for _, t := range b.transitions {
		if t.event == "" {
			return fmt.Errorf("fsm: transition registered with empty event")
		}
		if len(t.from) == 0 {
			return fmt.Errorf("fsm: transition for event %q has no From state", t.event)
		}
		if t.to == "" {
			return fmt.Errorf("fsm: transition for event %q has no To state", t.event)
		}
		for _, from := range t.from {
			key := string(t.event) + "\x00" + string(from)
			if _, dup := seen[key]; dup {
				return fmt.Errorf("fsm: duplicate transition for event %q from state %q", t.event, from)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

// buildTransitionIndex constructs a map[Event]map[State]*transitionDef for O(1) lookups.
// Called once during Build(); the result is immutable.
func buildTransitionIndex(transitions []*transitionDef) map[Event]map[State]*transitionDef {
	index := make(map[Event]map[State]*transitionDef)
	for _, t := range transitions {
		if index[t.event] == nil {
			index[t.event] = make(map[State]*transitionDef)
		}
		for _, from := range t.from {
			index[t.event][from] = t
		}
	}
	return index
}

// BeforeTransition registers a hook called before every transition (before guards run).
func (b *Builder) BeforeTransition(fn TransitionHookFunc) *Builder {
	b.hooks.before = append(b.hooks.before, fn)
	return b
}

// AfterTransition registers a hook called after every successful transition (after actions run).
func (b *Builder) AfterTransition(fn TransitionHookFunc) *Builder {
	b.hooks.after = append(b.hooks.after, fn)
	return b
}

// OnError registers a hook called when a guard or action returns an error.
func (b *Builder) OnError(fn ErrorHookFunc) *Builder {
	b.hooks.onErr = append(b.hooks.onErr, fn)
	return b
}

// OnEnter registers a hook called whenever the FSM enters the given state.
func (b *Builder) OnEnter(state State, fn StateHookFunc) *Builder {
	b.hooks.enter[state] = append(b.hooks.enter[state], fn)
	return b
}

// OnExit registers a hook called whenever the FSM exits the given state.
func (b *Builder) OnExit(state State, fn StateHookFunc) *Builder {
	b.hooks.exit[state] = append(b.hooks.exit[state], fn)
	return b
}

// WithHistory enables in-memory audit trail for the FSM.
// Use m.History() to read entries after Build().
func (b *Builder) WithHistory() *Builder {
	b.withHistory = true
	return b
}

// ---------------------------------------------------------------------------
// TransitionBuilder
// ---------------------------------------------------------------------------

// TransitionBuilder provides a fluent interface for configuring a single transition.
// It is returned by Builder.On and TransitionBuilder.On.
type TransitionBuilder struct {
	b   *Builder
	def *transitionDef
}

// From sets the source states for this transition.
// Multiple states can be provided; all of them will trigger the same target state.
func (tb *TransitionBuilder) From(states ...State) *TransitionBuilder {
	tb.def.from = append(tb.def.from, states...)
	return tb
}

// To sets the target state for this transition.
func (tb *TransitionBuilder) To(state State) *TransitionBuilder {
	tb.def.to = state
	return tb
}

// Guard adds a guard function to this transition.
// All guards are run in registration order; the first failure stops execution.
func (tb *TransitionBuilder) Guard(fn GuardFunc) *TransitionBuilder {
	tb.def.guards = append(tb.def.guards, fn)
	return tb
}

// Action adds an action function to this transition.
// Actions run in registration order after the state has changed.
func (tb *TransitionBuilder) Action(fn ActionFunc) *TransitionBuilder {
	tb.def.actions = append(tb.def.actions, fn)
	return tb
}

// On starts a new transition definition, returning to the parent Builder chain.
func (tb *TransitionBuilder) On(event Event) *TransitionBuilder {
	return tb.b.On(event)
}

// Build validates and returns the FSM, equivalent to calling Build on the parent Builder.
func (tb *TransitionBuilder) Build() (*FSM, error) {
	return tb.b.Build()
}

// BeforeTransition forwards to the parent Builder.
func (tb *TransitionBuilder) BeforeTransition(fn TransitionHookFunc) *Builder {
	return tb.b.BeforeTransition(fn)
}

// AfterTransition forwards to the parent Builder.
func (tb *TransitionBuilder) AfterTransition(fn TransitionHookFunc) *Builder {
	return tb.b.AfterTransition(fn)
}

// OnError forwards to the parent Builder.
func (tb *TransitionBuilder) OnError(fn ErrorHookFunc) *Builder {
	return tb.b.OnError(fn)
}

// OnEnter forwards to the parent Builder.
func (tb *TransitionBuilder) OnEnter(state State, fn StateHookFunc) *Builder {
	return tb.b.OnEnter(state, fn)
}

// OnExit forwards to the parent Builder.
func (tb *TransitionBuilder) OnExit(state State, fn StateHookFunc) *Builder {
	return tb.b.OnExit(state, fn)
}

// WithHistory forwards to the parent Builder.
func (tb *TransitionBuilder) WithHistory() *Builder {
	return tb.b.WithHistory()
}

// ---------------------------------------------------------------------------
// FSM runtime
// ---------------------------------------------------------------------------

// FSM is the runtime state machine. It is concurrent-safe by default; all
// exported methods are safe for use from multiple goroutines simultaneously.
//
// Guards and actions must not call back into the same FSM instance — they are
// invoked while the FSM holds its internal lock, which would cause a deadlock.
type FSM struct {
	mu      sync.RWMutex
	current State
	def     *definition
	hist    *history // nil when WithHistory() was not called
}

// NewWithState creates a runtime FSM from an existing definition and a restored
// state, e.g. a value loaded from a database. After a successful Trigger call
// the caller is responsible for persisting m.Current() back to storage.
//
// definition must come from a previous Builder.Build() call on the same machine
// schema. Passing an arbitrary State value that is not part of the schema is
// allowed and will surface naturally as ErrInvalidTransition on the first Trigger.
func NewWithState(def *FSM, savedState State) *FSM {
	f := &FSM{current: savedState, def: def.def}
	if def.def.withHistory {
		f.hist = &history{}
	}
	return f
}

// Current returns the current state.
func (f *FSM) Current() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.current
}

// Can reports whether the given event can be triggered from the current state.
// It does not change state.
func (f *FSM) Can(event Event) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, err := f.findTransition(f.current, event)
	return err == nil
}

// Transitions returns the events that can be triggered from the current state.
func (f *FSM) Transitions() []Event {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var events []Event
	for _, t := range f.def.transitions {
		for _, from := range t.from {
			if from == f.current {
				events = append(events, t.event)
				break
			}
		}
	}
	return events
}

// History returns a snapshot of all recorded state transitions in the order
// they occurred. Returns nil if WithHistory() was not called during Build.
func (f *FSM) History() []HistoryEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.hist == nil {
		return nil
	}
	return f.hist.snapshot()
}

// Trigger executes the given event against the current state.
//
// Execution order:
//  1. Validate that the transition exists for the current state and event.
//  2. Run BeforeTransition hooks.
//  3. Run OnExit hook for the source state.
//  4. Run all guards — if any guard fails, state is NOT changed.
//  5. Change state to the target state.
//  6. Run all actions — state has already changed at this point.
//  7. Run OnEnter hook for the target state.
//  8. Run AfterTransition hooks.
//
// On any guard or action failure, OnError hooks are called and the error is
// returned. BeforeTransition and OnExit still ran at that point.
func (f *FSM) Trigger(ctx context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, err := f.findTransition(f.current, event)
	if err != nil {
		return err
	}

	from := f.current
	to := t.to
	h := f.def.hooks

	// Step 2: BeforeTransition
	h.runBefore(ctx, from, to, event)

	// Step 3: OnExit source state
	h.runExit(ctx, from)

	// Step 4: Guards — state unchanged on any failure.
	for _, guard := range t.guards {
		if gErr := guard(ctx); gErr != nil {
			wrapped := &ErrGuardFailed{Cause: gErr}
			h.runOnError(ctx, wrapped)
			return wrapped
		}
	}

	// Step 5: Change state.
	f.current = to

	// Step 6: Actions — state already changed; errors do not roll back.
	for _, action := range t.actions {
		if aErr := action(ctx); aErr != nil {
			h.runOnError(ctx, aErr)
			return aErr
		}
	}

	// Step 7: OnEnter target state
	h.runEnter(ctx, to)

	// Step 8: AfterTransition
	h.runAfter(ctx, from, to, event)

	// Record history if enabled.
	if f.hist != nil {
		f.hist.record(from, to, event)
	}

	return nil
}

// findTransition looks up the matching transition for the given state and event.
// Caller must hold at least a read lock.
func (f *FSM) findTransition(state State, event Event) (*transitionDef, error) {
	// O(1) lookup via pre-built index
	stateMap, eventKnown := f.def.index[event]
	if !eventKnown {
		return nil, &ErrUnknownEvent{Event: event}
	}
	trans, validTransition := stateMap[state]
	if !validTransition {
		return nil, &ErrInvalidTransition{From: state, Event: event}
	}
	return trans, nil
}

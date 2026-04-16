package fsm_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/9edang/fsm"
)

// ---------------------------------------------------------------------------
// Domain constants used across tests
// ---------------------------------------------------------------------------

const (
	Pending   fsm.State = "pending"
	Confirmed fsm.State = "confirmed"
	Shipped   fsm.State = "shipped"
	Cancelled fsm.State = "cancelled"
)

// buildOrderFSM returns a fresh FSM for the order lifecycle:
//
//	pending  -[confirm]-> confirmed
//	confirmed -[ship]->   shipped
//	pending, confirmed -[cancel]-> cancelled
func buildOrderFSM(t *testing.T) *fsm.FSM {
	t.Helper()
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		On("ship").From(Confirmed).To(Shipped).
		On("cancel").From(Pending, Confirmed).To(Cancelled).
		Build()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	return m
}

// ---------------------------------------------------------------------------
// Phase 1 — Build validation
// ---------------------------------------------------------------------------

func TestBuild_Valid(t *testing.T) {
	_, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Build()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestBuild_Validation(t *testing.T) {
	tests := []struct {
		name    string
		build   func() (*fsm.FSM, error)
		wantErr string
	}{
		{
			name: "empty initial state",
			build: func() (*fsm.FSM, error) {
				return fsm.New("").
					On("confirm").From(Pending).To(Confirmed).
					Build()
			},
			wantErr: "initial state",
		},
		{
			name: "transition missing From",
			build: func() (*fsm.FSM, error) {
				return fsm.New(Pending).
					On("confirm").To(Confirmed).
					Build()
			},
			wantErr: "no From",
		},
		{
			name: "transition missing To",
			build: func() (*fsm.FSM, error) {
				return fsm.New(Pending).
					On("confirm").From(Pending).
					Build()
			},
			wantErr: "no To",
		},
		{
			name: "duplicate event from same state",
			build: func() (*fsm.FSM, error) {
				return fsm.New(Pending).
					On("confirm").From(Pending).To(Confirmed).
					On("confirm").From(Pending).To(Cancelled).
					Build()
			},
			wantErr: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.build()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 2 — Trigger: happy path
// ---------------------------------------------------------------------------

func TestTrigger_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		event     fsm.Event
		wantState fsm.State
	}{
		{"confirm from pending", "confirm", Confirmed},
		{"cancel from pending", "cancel", Cancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildOrderFSM(t)
			if err := m.Trigger(context.Background(), tt.event); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.Current() != tt.wantState {
				t.Errorf("state = %q, want %q", m.Current(), tt.wantState)
			}
		})
	}
}

func TestTrigger_MultiStep(t *testing.T) {
	m := buildOrderFSM(t)
	ctx := context.Background()

	steps := []struct {
		event fsm.Event
		want  fsm.State
	}{
		{"confirm", Confirmed},
		{"ship", Shipped},
	}
	for _, s := range steps {
		if err := m.Trigger(ctx, s.event); err != nil {
			t.Fatalf("trigger %q: %v", s.event, err)
		}
		if m.Current() != s.want {
			t.Errorf("after %q: state = %q, want %q", s.event, m.Current(), s.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 2 — Trigger: error cases
// ---------------------------------------------------------------------------

func TestTrigger_InvalidTransition(t *testing.T) {
	m := buildOrderFSM(t)
	// "ship" is only valid from Confirmed, not Pending
	err := m.Trigger(context.Background(), "ship")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var target *fsm.ErrInvalidTransition
	if !errors.As(err, &target) {
		t.Fatalf("expected *ErrInvalidTransition, got %T: %v", err, err)
	}
	if target.From != Pending {
		t.Errorf("From = %q, want %q", target.From, Pending)
	}
	if target.Event != "ship" {
		t.Errorf("Event = %q, want %q", target.Event, "ship")
	}
}

func TestTrigger_UnknownEvent(t *testing.T) {
	m := buildOrderFSM(t)
	err := m.Trigger(context.Background(), "refund")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var target *fsm.ErrUnknownEvent
	if !errors.As(err, &target) {
		t.Fatalf("expected *ErrUnknownEvent, got %T: %v", err, err)
	}
	if target.Event != "refund" {
		t.Errorf("Event = %q, want %q", target.Event, "refund")
	}
}

func TestTrigger_StateUnchanged_InvalidTransition(t *testing.T) {
	m := buildOrderFSM(t)
	_ = m.Trigger(context.Background(), "ship") // should fail
	if m.Current() != Pending {
		t.Errorf("state changed unexpectedly to %q", m.Current())
	}
}

func TestTrigger_StateUnchanged_UnknownEvent(t *testing.T) {
	m := buildOrderFSM(t)
	_ = m.Trigger(context.Background(), "unknown")
	if m.Current() != Pending {
		t.Errorf("state changed unexpectedly to %q", m.Current())
	}
}

// ---------------------------------------------------------------------------
// Phase 3 — Guards
// ---------------------------------------------------------------------------

func TestGuard_Pass(t *testing.T) {
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if m.Current() != Confirmed {
		t.Errorf("state = %q, want %q", m.Current(), Confirmed)
	}
}

func TestGuard_Fail_StateUnchanged(t *testing.T) {
	guardErr := errors.New("payment not verified")
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { return guardErr }).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	trigErr := m.Trigger(context.Background(), "confirm")
	if trigErr == nil {
		t.Fatal("expected error, got nil")
	}

	var target *fsm.ErrGuardFailed
	if !errors.As(trigErr, &target) {
		t.Fatalf("expected *ErrGuardFailed, got %T: %v", trigErr, trigErr)
	}
	if !errors.Is(trigErr, guardErr) {
		t.Errorf("expected wrapped cause %v, got %v", guardErr, target.Cause)
	}
	if m.Current() != Pending {
		t.Errorf("state changed to %q after guard failure", m.Current())
	}
}

func TestGuard_MultipleGuards_OrderPreserved(t *testing.T) {
	var order []int
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { order = append(order, 1); return nil }).
		Guard(func(ctx context.Context) error { order = append(order, 2); return nil }).
		Guard(func(ctx context.Context) error { order = append(order, 3); return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	for i, v := range order {
		if v != i+1 {
			t.Errorf("guard order[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestGuard_StopsAtFirstFailure(t *testing.T) {
	var called []int
	failErr := errors.New("fail at guard 1")
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { called = append(called, 1); return failErr }).
		Guard(func(ctx context.Context) error { called = append(called, 2); return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if len(called) != 1 {
		t.Errorf("expected 1 guard called, got %d: %v", len(called), called)
	}
}

// ---------------------------------------------------------------------------
// Phase 4 — Actions
// ---------------------------------------------------------------------------

func TestAction_ExecutedAfterTransition(t *testing.T) {
	var actionRan bool
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Action(func(ctx context.Context) error {
			actionRan = true
			return nil
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	if !actionRan {
		t.Error("expected action to run, but it did not")
	}
}

func TestAction_MultipleActions_OrderPreserved(t *testing.T) {
	var order []int
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Action(func(ctx context.Context) error { order = append(order, 1); return nil }).
		Action(func(ctx context.Context) error { order = append(order, 2); return nil }).
		Action(func(ctx context.Context) error { order = append(order, 3); return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	for i, v := range order {
		if v != i+1 {
			t.Errorf("action order[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestAction_StateAlreadyChangedOnActionError(t *testing.T) {
	// By design: state changes before actions run.
	// If an action fails, state has already changed.
	actionErr := errors.New("email delivery failed")
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Action(func(ctx context.Context) error { return actionErr }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	err = m.Trigger(context.Background(), "confirm")
	if err == nil {
		t.Fatal("expected action error")
	}
	if !errors.Is(err, actionErr) {
		t.Errorf("expected %v, got %v", actionErr, err)
	}
	// State has already changed despite the action error — this is documented behaviour.
	if m.Current() != Confirmed {
		t.Errorf("expected state %q after action error, got %q", Confirmed, m.Current())
	}
}

// ---------------------------------------------------------------------------
// Phase 5 — State Query
// ---------------------------------------------------------------------------

func TestCurrent_ReturnsInitialState(t *testing.T) {
	m := buildOrderFSM(t)
	if m.Current() != Pending {
		t.Errorf("Current() = %q, want %q", m.Current(), Pending)
	}
}

func TestCan(t *testing.T) {
	tests := []struct {
		name  string
		event fsm.Event
		want  bool
	}{
		{"can confirm from pending", "confirm", true},
		{"can cancel from pending", "cancel", true},
		{"cannot ship from pending", "ship", false},
		{"unknown event returns false", "refund", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildOrderFSM(t)
			if got := m.Can(tt.event); got != tt.want {
				t.Errorf("Can(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestCan_DoesNotChangeState(t *testing.T) {
	m := buildOrderFSM(t)
	_ = m.Can("confirm")
	if m.Current() != Pending {
		t.Errorf("Can() changed state to %q", m.Current())
	}
}

func TestTransitions_FromPending(t *testing.T) {
	m := buildOrderFSM(t)
	got := m.Transitions()
	want := map[fsm.Event]bool{"confirm": true, "cancel": true}

	if len(got) != len(want) {
		t.Fatalf("Transitions() = %v, want %d events", got, len(want))
	}
	for _, e := range got {
		if !want[e] {
			t.Errorf("unexpected event %q in Transitions()", e)
		}
	}
}

func TestTransitions_FromConfirmed(t *testing.T) {
	m := buildOrderFSM(t)
	_ = m.Trigger(context.Background(), "confirm")

	got := m.Transitions()
	want := map[fsm.Event]bool{"ship": true, "cancel": true}

	if len(got) != len(want) {
		t.Fatalf("Transitions() = %v, want %d events", got, len(want))
	}
	for _, e := range got {
		if !want[e] {
			t.Errorf("unexpected event %q in Transitions()", e)
		}
	}
}

func TestTransitions_Terminal(t *testing.T) {
	m := buildOrderFSM(t)
	ctx := context.Background()
	_ = m.Trigger(ctx, "confirm")
	_ = m.Trigger(ctx, "ship")

	if got := m.Transitions(); len(got) != 0 {
		t.Errorf("expected no transitions from terminal state, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Phase 6 — Error types: errors.As / errors.Is
// ---------------------------------------------------------------------------

func TestErrorTypes_ErrorAs(t *testing.T) {
	m := buildOrderFSM(t)
	ctx := context.Background()

	t.Run("ErrInvalidTransition", func(t *testing.T) {
		err := m.Trigger(ctx, "ship")
		var target *fsm.ErrInvalidTransition
		if !errors.As(err, &target) {
			t.Errorf("expected *ErrInvalidTransition, got %T: %v", err, err)
		}
	})

	t.Run("ErrUnknownEvent", func(t *testing.T) {
		err := m.Trigger(ctx, "refund")
		var target *fsm.ErrUnknownEvent
		if !errors.As(err, &target) {
			t.Errorf("expected *ErrUnknownEvent, got %T: %v", err, err)
		}
	})

	t.Run("ErrGuardFailed wraps cause", func(t *testing.T) {
		cause := errors.New("guard reason")
		m2, _ := fsm.New(Pending).
			On("confirm").From(Pending).To(Confirmed).
			Guard(func(ctx context.Context) error { return cause }).
			Build()
		err := m2.Trigger(ctx, "confirm")

		var target *fsm.ErrGuardFailed
		if !errors.As(err, &target) {
			t.Errorf("expected *ErrGuardFailed, got %T: %v", err, err)
		}
		if !errors.Is(err, cause) {
			t.Errorf("errors.Is: expected %v to be in chain, got %v", cause, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Phase 7 — Concurrent-safe (race detector must pass with -race)
// ---------------------------------------------------------------------------

func TestConcurrency_IndependentInstances(t *testing.T) {
	// Each goroutine owns its own FSM — no shared state.
	const goroutines = 50
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := buildOrderFSM(t)
			if err := m.Trigger(context.Background(), "confirm"); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("goroutine trigger error: %v", err)
	}
}

func TestConcurrency_SharedInstance_NoRace(t *testing.T) {
	// All goroutines share one FSM. Only one will succeed at "confirm";
	// others get ErrInvalidTransition. We only care that there are no data races.
	m := buildOrderFSM(t)
	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = m.Current()
			_ = m.Can("confirm")
			_ = m.Transitions()
			_ = m.Trigger(ctx, "confirm") // most will fail — that is expected
		}()
	}
	wg.Wait()
}

func TestConcurrency_ReadsDuringTrigger(t *testing.T) {
	m := buildOrderFSM(t)
	var wg sync.WaitGroup

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = m.Trigger(context.Background(), "confirm")
		}
	}()

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.Current()
				_ = m.Can("ship")
				_ = m.Transitions()
			}
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Phase 9 — Global Hooks
// ---------------------------------------------------------------------------

func TestBeforeTransition_Called(t *testing.T) {
	var calls []string
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		BeforeTransition(func(ctx context.Context, from, to fsm.State, event fsm.Event) {
			calls = append(calls, "before:"+string(from)+"->"+string(to))
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "before:pending->confirmed" {
		t.Errorf("BeforeTransition calls = %v", calls)
	}
}

func TestAfterTransition_Called(t *testing.T) {
	var calls []string
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		AfterTransition(func(ctx context.Context, from, to fsm.State, event fsm.Event) {
			calls = append(calls, "after:"+string(from)+"->"+string(to))
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "after:pending->confirmed" {
		t.Errorf("AfterTransition calls = %v", calls)
	}
}

func TestOnError_CalledOnGuardFailure(t *testing.T) {
	var errReceived error
	guardErr := errors.New("payment not verified")
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { return guardErr }).
		OnError(func(ctx context.Context, e error) { errReceived = e }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if errReceived == nil {
		t.Fatal("expected OnError to be called")
	}
	if !errors.Is(errReceived, guardErr) {
		t.Errorf("OnError received %v, want to contain %v", errReceived, guardErr)
	}
}

func TestOnError_CalledOnActionFailure(t *testing.T) {
	var errReceived error
	actionErr := errors.New("send email failed")
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Action(func(ctx context.Context) error { return actionErr }).
		OnError(func(ctx context.Context, e error) { errReceived = e }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if !errors.Is(errReceived, actionErr) {
		t.Errorf("OnError received %v, want %v", errReceived, actionErr)
	}
}

// ---------------------------------------------------------------------------
// Phase 10 — OnEnter / OnExit Hooks
// ---------------------------------------------------------------------------

func TestOnEnter_CalledForTargetState(t *testing.T) {
	var entered []fsm.State
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		OnEnter(Confirmed, func(ctx context.Context) { entered = append(entered, Confirmed) }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	if len(entered) != 1 || entered[0] != Confirmed {
		t.Errorf("OnEnter called with %v, want [%q]", entered, Confirmed)
	}
}

func TestOnEnter_NotCalledForOtherState(t *testing.T) {
	var entered []fsm.State
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		OnEnter(Shipped, func(ctx context.Context) { entered = append(entered, Shipped) }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if len(entered) != 0 {
		t.Errorf("OnEnter for Shipped should not have been called, got %v", entered)
	}
}

func TestOnExit_CalledForSourceState(t *testing.T) {
	var exited []fsm.State
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		OnExit(Pending, func(ctx context.Context) { exited = append(exited, Pending) }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	if len(exited) != 1 || exited[0] != Pending {
		t.Errorf("OnExit called with %v, want [%q]", exited, Pending)
	}
}

func TestOnExit_CalledBeforeGuard(t *testing.T) {
	// OnExit fires before guards. State is NOT changed when guard fails.
	var exitCalled bool
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { return errors.New("blocked") }).
		OnExit(Pending, func(ctx context.Context) { exitCalled = true }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if !exitCalled {
		t.Error("expected OnExit to be called before guard ran")
	}
	if m.Current() != Pending {
		t.Errorf("state changed to %q despite guard failure", m.Current())
	}
}

func TestHook_FullExecutionOrder(t *testing.T) {
	// Verifies: BeforeTransition → OnExit → Guard → state change →
	//           Action → OnEnter → AfterTransition
	var order []string
	m, err := fsm.New(Pending).
		BeforeTransition(func(ctx context.Context, from, to fsm.State, event fsm.Event) {
			order = append(order, "before")
		}).
		AfterTransition(func(ctx context.Context, from, to fsm.State, event fsm.Event) {
			order = append(order, "after")
		}).
		OnEnter(Confirmed, func(ctx context.Context) { order = append(order, "enter") }).
		OnExit(Pending, func(ctx context.Context) { order = append(order, "exit") }).
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { order = append(order, "guard"); return nil }).
		Action(func(ctx context.Context) error { order = append(order, "action"); return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}

	want := []string{"before", "exit", "guard", "action", "enter", "after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 11 — Persistence / NewWithState
// ---------------------------------------------------------------------------

func TestNewWithState_RestoresFromDB(t *testing.T) {
	template, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		On("ship").From(Confirmed).To(Shipped).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := fsm.NewWithState(template, Confirmed)
	if m.Current() != Confirmed {
		t.Errorf("Current() = %q, want %q", m.Current(), Confirmed)
	}
	if err := m.Trigger(context.Background(), "ship"); err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
	if m.Current() != Shipped {
		t.Errorf("after ship: Current() = %q, want %q", m.Current(), Shipped)
	}
}

func TestNewWithState_TemplateUnchanged(t *testing.T) {
	template, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := fsm.NewWithState(template, Pending)
	_ = m.Trigger(context.Background(), "confirm")

	if template.Current() != Pending {
		t.Errorf("template state was mutated to %q", template.Current())
	}
}

func TestNewWithState_IntegrationOrderLifecycle(t *testing.T) {
	type fakeDB struct{ state fsm.State }
	db := &fakeDB{state: Pending}

	template, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		On("ship").From(Confirmed).To(Shipped).
		On("cancel").From(Pending, Confirmed).To(Cancelled).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	steps := []struct {
		event fsm.Event
		want  fsm.State
	}{
		{"confirm", Confirmed},
		{"ship", Shipped},
	}

	for _, s := range steps {
		m := fsm.NewWithState(template, db.state)
		if err := m.Trigger(ctx, s.event); err != nil {
			t.Fatalf("event %q failed: %v", s.event, err)
		}
		db.state = m.Current()
		if db.state != s.want {
			t.Errorf("after %q: db.state = %q, want %q", s.event, db.state, s.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 14 — History / Audit Trail
// ---------------------------------------------------------------------------

func TestHistory_RecordsEntries(t *testing.T) {
	m, err := fsm.New(Pending).
		WithHistory().
		On("confirm").From(Pending).To(Confirmed).
		On("ship").From(Confirmed).To(Shipped).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Trigger(ctx, "confirm")
	_ = m.Trigger(ctx, "ship")

	h := m.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(h))
	}
	if h[0].From != Pending || h[0].To != Confirmed || h[0].Event != "confirm" {
		t.Errorf("entry[0] = %+v", h[0])
	}
	if h[1].From != Confirmed || h[1].To != Shipped || h[1].Event != "ship" {
		t.Errorf("entry[1] = %+v", h[1])
	}
}

func TestHistory_NilWhenDisabled(t *testing.T) {
	m := buildOrderFSM(t)
	_ = m.Trigger(context.Background(), "confirm")
	if h := m.History(); h != nil {
		t.Errorf("expected nil history when WithHistory() not called, got %v", h)
	}
}

func TestHistory_NotRecordedOnGuardFailure(t *testing.T) {
	m, err := fsm.New(Pending).
		WithHistory().
		On("confirm").From(Pending).To(Confirmed).
		Guard(func(ctx context.Context) error { return errors.New("blocked") }).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	if h := m.History(); len(h) != 0 {
		t.Errorf("expected no history entries after guard failure, got %v", h)
	}
}

func TestHistory_SnapshotIsImmutable(t *testing.T) {
	m, err := fsm.New(Pending).
		WithHistory().
		On("confirm").From(Pending).To(Confirmed).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Trigger(context.Background(), "confirm")
	snap1 := m.History()
	snap1[0].Event = "tampered"

	snap2 := m.History()
	if snap2[0].Event == "tampered" {
		t.Error("History() snapshot is not isolated — mutation affected internal state")
	}
}

// ---------------------------------------------------------------------------
// Phase 13 — Export (ToMermaid / ToDOT)
// ---------------------------------------------------------------------------

func TestToMermaid_ContainsAllTransitions(t *testing.T) {
	m := buildOrderFSM(t)
	out := m.ToMermaid()

	must := []string{
		"stateDiagram-v2",
		"--> pending",
		"pending --> confirmed",
		"confirmed --> shipped",
		"pending --> cancelled",
		"confirmed --> cancelled",
	}
	for _, s := range must {
		if !strings.Contains(out, s) {
			t.Errorf("ToMermaid() missing %q\nFull output:\n%s", s, out)
		}
	}
}

func TestToDOT_ContainsAllTransitions(t *testing.T) {
	m := buildOrderFSM(t)
	out := m.ToDOT()

	must := []string{
		"digraph fsm",
		"__start__",
		"pending",
		"confirmed",
		"shipped",
		"cancelled",
		`"confirm"`,
		`"ship"`,
		`"cancel"`,
	}
	for _, s := range must {
		if !strings.Contains(out, s) {
			t.Errorf("ToDOT() missing %q\nFull output:\n%s", s, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Coverage: error .Error() string methods
// ---------------------------------------------------------------------------

func TestErrorMessages(t *testing.T) {
	t.Run("ErrInvalidTransition", func(t *testing.T) {
		e := &fsm.ErrInvalidTransition{From: "pending", Event: "ship"}
		msg := e.Error()
		if !strings.Contains(msg, "ship") || !strings.Contains(msg, "pending") {
			t.Errorf("unexpected message: %q", msg)
		}
	})

	t.Run("ErrGuardFailed", func(t *testing.T) {
		cause := errors.New("not allowed")
		e := &fsm.ErrGuardFailed{Cause: cause}
		msg := e.Error()
		if !strings.Contains(msg, "not allowed") {
			t.Errorf("unexpected message: %q", msg)
		}
	})

	t.Run("ErrUnknownEvent", func(t *testing.T) {
		e := &fsm.ErrUnknownEvent{Event: "refund"}
		msg := e.Error()
		if !strings.Contains(msg, "refund") {
			t.Errorf("unexpected message: %q", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// Coverage: validateBuilder — empty event name branch
// ---------------------------------------------------------------------------

func TestBuild_Validation_EmptyEvent(t *testing.T) {
	// Directly registering an empty event via On("") exercises the
	// "transition registered with empty event" branch in validateBuilder.
	_, err := fsm.New(Pending).
		On("").From(Pending).To(Confirmed).
		Build()
	if err == nil {
		t.Fatal("expected error for empty event name, got nil")
	}
	if !strings.Contains(err.Error(), "empty event") {
		t.Errorf("error %q does not mention empty event", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Coverage: TransitionBuilder.WithHistory forwarding method
// ---------------------------------------------------------------------------

func TestTransitionBuilder_WithHistory(t *testing.T) {
	// Chain .WithHistory() from a TransitionBuilder (not directly from Builder)
	// to exercise the forwarding method on TransitionBuilder.
	m, err := fsm.New(Pending).
		On("confirm").From(Pending).To(Confirmed).
		WithHistory().
		On("ship").From(Confirmed).To(Shipped).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = m.Trigger(ctx, "confirm")
	h := m.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(h))
	}
}

// ---------------------------------------------------------------------------
// Coverage: NewWithState with WithHistory enabled
// ---------------------------------------------------------------------------

func TestNewWithState_WithHistory(t *testing.T) {
	// Build a template that has history enabled, then restore it via NewWithState.
	// This exercises the `if def.def.withHistory { f.hist = &history{} }` branch.
	template, err := fsm.New(Pending).
		WithHistory().
		On("confirm").From(Pending).To(Confirmed).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	m := fsm.NewWithState(template, Pending)
	if err := m.Trigger(context.Background(), "confirm"); err != nil {
		t.Fatal(err)
	}
	h := m.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history entry after NewWithState trigger, got %d", len(h))
	}
	if h[0].From != Pending || h[0].To != Confirmed {
		t.Errorf("unexpected history entry: %+v", h[0])
	}
}

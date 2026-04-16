package fsm_test

import (
	"context"
	"testing"

	"github.com/9edang/fsm"
)

// buildBenchFSM returns a simple two-state FSM suitable for hot-path benchmarks.
// pending → active → done, with a cancel from both.
func buildBenchFSM(b *testing.B) *fsm.FSM {
	b.Helper()
	m, err := fsm.New("pending").
		On("start").From("pending").To("active").
		On("finish").From("active").To("done").
		Build()
	if err != nil {
		b.Fatal(err)
	}
	return m
}

// BenchmarkTrigger_HotPath measures the cost of a single successful Trigger call
// on a pre-positioned FSM (no guards, no actions, no hooks).
func BenchmarkTrigger_HotPath(b *testing.B) {
	ctx := context.Background()

	// Build the template once; restore to "pending" each iteration via NewWithState.
	template := buildBenchFSM(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := fsm.NewWithState(template, "pending")
		_ = m.Trigger(ctx, "start")
	}
}

// BenchmarkTrigger_WithGuardAndAction measures Trigger with one guard and one action.
func BenchmarkTrigger_WithGuardAndAction(b *testing.B) {
	ctx := context.Background()

	template, err := fsm.New("pending").
		On("start").From("pending").To("active").
			Guard(func(_ context.Context) error { return nil }).
			Action(func(_ context.Context) error { return nil }).
		Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := fsm.NewWithState(template, "pending")
		_ = m.Trigger(ctx, "start")
	}
}

// BenchmarkTrigger_WithHooks measures Trigger with all hook types registered.
func BenchmarkTrigger_WithHooks(b *testing.B) {
	ctx := context.Background()

	noop := func(_ context.Context, _, _ fsm.State, _ fsm.Event) {}
	noopState := func(_ context.Context) {}

	template, err := fsm.New("pending").
		On("start").From("pending").To("active").
		BeforeTransition(noop).
		AfterTransition(noop).
		OnEnter("active", noopState).
		OnExit("pending", noopState).
		Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := fsm.NewWithState(template, "pending")
		_ = m.Trigger(ctx, "start")
	}
}

// BenchmarkTrigger_Parallel measures concurrent Trigger throughput.
// Each goroutine gets its own FSM instance (no shared state contention).
func BenchmarkTrigger_Parallel(b *testing.B) {
	ctx := context.Background()
	template := buildBenchFSM(b)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := fsm.NewWithState(template, "pending")
			_ = m.Trigger(ctx, "start")
		}
	})
}

// BenchmarkCan measures the read-only Can check.
func BenchmarkCan(b *testing.B) {
	m := buildBenchFSM(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Can("start")
	}
}

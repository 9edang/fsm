package fsm

import "fmt"

// ErrInvalidTransition is returned when the event is known but not allowed from the current state.
type ErrInvalidTransition struct {
	From  State
	Event Event
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("fsm: cannot trigger %q from state %q", string(e.Event), string(e.From))
}

// ErrGuardFailed is returned when a guard rejects the transition.
// The original guard error is accessible via Unwrap.
type ErrGuardFailed struct {
	Cause error
}

func (e *ErrGuardFailed) Error() string {
	return fmt.Sprintf("fsm: guard rejected transition: %s", e.Cause)
}

func (e *ErrGuardFailed) Unwrap() error {
	return e.Cause
}

// ErrUnknownEvent is returned when the event is not registered in the FSM at all.
type ErrUnknownEvent struct {
	Event Event
}

func (e *ErrUnknownEvent) Error() string {
	return fmt.Sprintf("fsm: unknown event %q", string(e.Event))
}

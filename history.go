package fsm

import "time"

// HistoryEntry records a single successful state transition.
type HistoryEntry struct {
	From  State
	To    State
	Event Event
	At    time.Time
}

// history is the in-memory audit trail stored inside an FSM instance.
// It is only allocated when WithHistory() is called during Build.
type history struct {
	entries []HistoryEntry
}

func (h *history) record(from, to State, event Event) {
	h.entries = append(h.entries, HistoryEntry{
		From:  from,
		To:    to,
		Event: event,
		At:    time.Now(),
	})
}

// snapshot returns a copy of the entries slice so callers cannot mutate it.
func (h *history) snapshot() []HistoryEntry {
	out := make([]HistoryEntry, len(h.entries))
	copy(out, h.entries)
	return out
}

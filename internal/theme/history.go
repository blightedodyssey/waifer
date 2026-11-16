package theme

// HistoryManager is a bounded undo stack of ThemeState snapshots.
type HistoryManager struct {
	entries []ThemeState
	limit   int
}

// NewHistoryManager returns an empty history with a default capacity.
func NewHistoryManager() *HistoryManager {
	return &HistoryManager{limit: 64}
}

// Push records a state snapshot, dropping the oldest entry when full.
func (h *HistoryManager) Push(state ThemeState) {
	if h.limit <= 0 {
		h.limit = 64
	}
	if len(h.entries) >= h.limit {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, state)
}

// Pop removes and returns the most recent snapshot.
func (h *HistoryManager) Pop() (ThemeState, bool) {
	if len(h.entries) == 0 {
		return ThemeState{}, false
	}
	state := h.entries[len(h.entries)-1]
	h.entries = h.entries[:len(h.entries)-1]
	return state, true
}

// Len reports how many snapshots are recorded.
func (h *HistoryManager) Len() int {
	return len(h.entries)
}

// Clear forgets all snapshots.
func (h *HistoryManager) Clear() {
	h.entries = nil
}

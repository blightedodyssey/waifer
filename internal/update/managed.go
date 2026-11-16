package update

import "sync"

// managedState guards one-at-a-time upgrades within a process.
type managedState struct {
	mu      sync.Mutex
	running bool
}

// state is the process-level upgrade state.
var state = &managedState{}

// tryBeginUpgrade claims the upgrade slot.
func tryBeginUpgrade() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.running {
		return false
	}
	state.running = true
	return true
}

// endUpgrade releases the upgrade slot.
func endUpgrade() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.running = false
}

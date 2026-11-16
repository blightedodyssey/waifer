//go:build !windows

package tray

func newNoop() <-chan Cmd {
	ch := make(chan Cmd)
	close(ch)
	return ch
}

func stopNoop() {}

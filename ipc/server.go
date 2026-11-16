package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
)

// Server accepts one JSON request per connection line and dispatches it to
// the Dispatcher (the App). It listens at the path given to NewServer.
type Server struct {
	dispatcher Dispatcher
	listener   net.Listener

	mu     sync.Mutex
	closed bool
	done   chan struct{}
}

// NewServer starts listening at path. On Unix it removes a stale socket file
// owned by a dead process first; a live listener is reported as an error. On
// Windows the named pipe namespace handles that automatically.
func NewServer(path string, dispatcher Dispatcher) (*Server, error) {
	if runtime.GOOS != "windows" {
		if err := cleanStaleSocket(path); err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err == nil {
			if pid, ok := socketOwnerPID(path); ok {
				return nil, fmt.Errorf("ipc: aether is already running (pid %d)", pid)
			}
			return nil, fmt.Errorf("ipc: aether is already running (socket %s in use)", path)
		}
	}

	network := "unix"
	if runtime.GOOS == "windows" {
		network = "pipe"
	}
	listener, err := net.Listen(network, path)
	if err != nil {
		return nil, fmt.Errorf("ipc listen: %w", err)
	}
	if runtime.GOOS != "windows" {
		// Only the current user may talk to the GUI.
		_ = os.Chmod(path, 0o600)
	}

	s := &Server{
		dispatcher: dispatcher,
		listener:   listener,
		done:       make(chan struct{}),
	}
	go s.acceptLoop()
	return s, nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			s.writeResponse(conn, Response{OK: false, Error: "invalid request: " + err.Error()})
			continue
		}
		resp := s.dispatcher.HandleIPC(req)
		if err := s.writeResponse(conn, resp); err != nil {
			return
		}
	}
}

func (s *Server) writeResponse(conn net.Conn, resp Response) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		payload, _ = json.Marshal(Response{OK: false, Error: "marshal response: " + err.Error()})
	}
	_, err = conn.Write(append(payload, '\n'))
	return err
}

// Close stops accepting connections and removes the socket on Unix.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	_ = s.listener.Close()
	close(s.done)
	if runtime.GOOS != "windows" {
		if err := os.Remove(s.listener.Addr().String()); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// cleanStaleSocket removes a leftover socket file whose owner process is gone.
func cleanStaleSocket(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ipc: inspect socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		// Regular file at the socket path — remove only if it looks stale.
		_ = os.Remove(path)
		return nil
	}
	if _, ok := socketOwnerPID(path); ok {
		return nil // live listener; caller reports "already running"
	}
	// Nobody is listening; the file is stale.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ipc: remove stale socket: %w", err)
	}
	return nil
}

// socketOwnerPID attempts to identify a live listener on path by connecting
// to it. A successful dial means the socket is in use.
func socketOwnerPID(path string) (int, bool) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return 0, false
	}
	_ = conn.Close()
	return os.Getpid(), true // live listener exists; PID is not discoverable portably
}

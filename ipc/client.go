package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"runtime"

	"aether/internal/platform"
)

// socketName is the pipe/socket base name shared by client and server.
const socketName = "aether.sock"

// DefaultSocketPath returns the IPC endpoint for this user. On Windows that
// is a named pipe under \\.\pipe\; elsewhere it is a Unix socket inside the
// Aether config directory.
func DefaultSocketPath() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\` + socketName
	}
	return filepath.Join(platform.ConfigDir(), socketName)
}

// IsRunning reports whether another Aether instance is accepting IPC
// connections at the default endpoint.
func IsRunning() bool {
	conn, err := dialSocket(DefaultSocketPath())
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Send delivers req to the server listening at path and returns its response.
func Send(path string, req Request) (Response, error) {
	var zero Response
	conn, err := dialSocket(path)
	if err != nil {
		return zero, fmt.Errorf("aether is not running (no socket at %s): %w", path, err)
	}
	defer conn.Close()

	payload, err := json.Marshal(req)
	if err != nil {
		return zero, fmt.Errorf("encode request: %w", err)
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return zero, fmt.Errorf("send request: %w", err)
	}

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return zero, fmt.Errorf("read response: %w", err)
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

func dialSocket(path string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return net.Dial("pipe", path)
	}
	return net.Dial("unix", path)
}

var errNoConnection = errors.New("ipc connection failed")

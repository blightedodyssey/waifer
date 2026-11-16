// Package ipc implements the JSON-line protocol used by the CLI (`aether
// status`, `aether extract`, ...) to control a running Aether GUI instance.
package ipc

import "encoding/json"

// Request is one command sent from the CLI to the GUI. Only the fields
// relevant to the command are populated; the GUI's HandleIPC switch decides
// which ones apply.
type Request struct {
	Cmd  string `json:"cmd"`
	Path string `json:"path,omitempty"`
	Mode string `json:"mode,omitempty"`
	Name string `json:"name,omitempty"`

	LightMode *bool `json:"lightMode,omitempty"`

	Index int    `json:"index,omitempty"`
	Value string `json:"value,omitempty"`

	Palette []string `json:"palette,omitempty"`

	// Optional adjustment deltas for "adjust". nil means "leave unchanged".
	Vibrance   *float64 `json:"vibrance,omitempty"`
	Saturation *float64 `json:"saturation,omitempty"`
	Contrast   *float64 `json:"contrast,omitempty"`
	Brightness *float64 `json:"brightness,omitempty"`
	Shadows    *float64 `json:"shadows,omitempty"`
	Highlights *float64 `json:"highlights,omitempty"`
	HueShift   *float64 `json:"hueShift,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	Tint       *float64 `json:"tint,omitempty"`
	Gamma      *float64 `json:"gamma,omitempty"`
	BlackPoint *float64 `json:"blackPoint,omitempty"`
	WhitePoint *float64 `json:"whitePoint,omitempty"`
}

// Response is the GUI's reply. Data carries arbitrary JSON payloads (e.g. the
// blueprints list for "list-blueprints", the variable map for
// "get-variables") as pre-marshaled JSON.
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`

	Mode      string   `json:"mode,omitempty"`
	LightMode *bool    `json:"lightMode,omitempty"`
	Wallpaper string   `json:"wallpaper,omitempty"`
	Palette   []string `json:"palette,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Dispatcher receives parsed requests from the server. The GUI's App
// implements this.
type Dispatcher interface {
	HandleIPC(req Request) Response
}

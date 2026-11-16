package extraction

import (
	"encoding/json"
	"time"
)

// wrapper decorates cached palettes with a timestamp.
type wrapper struct {
	Timestamp int64      `json:"timestamp"`
	Palette   [16]string `json:"palette"`
}

// marshalPalette encodes a palette for disk.
func marshalPalette(palette [16]string) ([]byte, error) {
	return json.Marshal(wrapper{Timestamp: time.Now().Unix(), Palette: palette})
}

// unmarshalPalette decodes a disk palette.
func unmarshalPalette(data []byte, out *[16]string) error {
	var w wrapper
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*out = w.Palette
	return nil
}

// stale reports whether a cached document is older than the TTL.
func stale(data []byte) bool {
	var w wrapper
	if err := json.Unmarshal(data, &w); err != nil {
		return true
	}
	return w.Timestamp+cacheTTLSeconds < time.Now().Unix()
}

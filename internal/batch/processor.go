// Package batch processes a list of wallpapers in the background, extracting
// a palette preview for each and reporting progress to the frontend.
package batch

import (
	"context"
	"sync"

	"aether/internal/extraction"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Result is the per-file outcome reported to the frontend.
type Result struct {
	Path    string     `json:"path"`
	Palette [16]string `json:"palette,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// Processor runs batch extraction jobs one at a time.
type Processor struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// NewProcessor returns an idle processor.
func NewProcessor() *Processor {
	return &Processor{}
}

// Start launches a batch extraction over paths. A previously running batch
// is cancelled first. Events:
//
//	batch-started   {total}
//	batch-progress  {path, index, total, palette}
//	batch-complete  {results}
func (p *Processor) Start(ctx context.Context, paths []string, lightMode bool) {
	if ctx == nil || len(paths) == 0 {
		return
	}
	p.Cancel()

	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.running = true
	p.cancel = cancel
	p.mu.Unlock()

	go func() {
		defer func() {
			p.mu.Lock()
			p.running = false
			p.cancel = nil
			p.mu.Unlock()
		}()

		wailsrt.EventsEmit(ctx, "batch-started", map[string]interface{}{"total": len(paths)})
		results := make([]Result, 0, len(paths))
		for i, path := range paths {
			if err := runCtx.Err(); err != nil {
				break
			}
			result := Result{Path: path}
			mode := "normal"
			palette, err := extraction.ExtractColors(path, lightMode, mode)
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Palette = palette
			}
			results = append(results, result)
			wailsrt.EventsEmit(ctx, "batch-progress", map[string]interface{}{
				"path":    path,
				"index":   i + 1,
				"total":   len(paths),
				"palette": palette[:],
				"error":   result.Error,
			})
		}
		wailsrt.EventsEmit(ctx, "batch-complete", map[string]interface{}{"results": results})
	}()
}

// Cancel stops the running batch, if any.
func (p *Processor) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
}

// IsRunning reports whether a batch is in flight.
func (p *Processor) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

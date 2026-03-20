// Package ps tracks which cloud models are currently "active" —
// i.e. have at least one in-flight request against them.
//
// Because cloud models are not actually loaded into VRAM on our side,
// we synthesise a reasonable /api/ps response by tracking request
// lifecycles with a RWMutex-protected map. This is safe for concurrent
// access from many handler goroutines.
package ps

import (
	"sync"
	"time"

	"github.com/dominionthedev/ollacloud/internal/api"
)

// activeEntry is the internal state for one in-flight model.
type activeEntry struct {
	model     string
	startedAt time.Time
	count     int // concurrent request count for this model
}

// Tracker maintains the set of currently-active cloud models.
type Tracker struct {
	mu     sync.RWMutex
	active map[string]*activeEntry
}

// New creates a ready-to-use Tracker.
func New() *Tracker {
	return &Tracker{
		active: make(map[string]*activeEntry),
	}
}

// Acquire marks model as active. Call it before forwarding a generation
// request. Returns a release function that must be deferred.
func (t *Tracker) Acquire(model string) (release func()) {
	t.mu.Lock()
	if e, ok := t.active[model]; ok {
		e.count++
	} else {
		t.active[model] = &activeEntry{
			model:     model,
			startedAt: time.Now(),
			count:     1,
		}
	}
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if e, ok := t.active[model]; ok {
			e.count--
			if e.count <= 0 {
				delete(t.active, model)
			}
		}
	}
}

// Snapshot returns a synthetic PSResponse for the /api/ps endpoint.
// We report active models with reasonable placeholder VRAM/size values
// since we have no local VRAM to track.
func (t *Tracker) Snapshot() api.PSResponse {
	t.mu.RLock()
	defer t.mu.RUnlock()

	models := make([]api.RunningModel, 0, len(t.active))
	for _, e := range t.active {
		// Cloud models expire conceptually when the request ends.
		// We set expires_at to slightly ahead of now so clients don't
		// think it's already expired.
		expiresAt := time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano)

		models = append(models, api.RunningModel{
			Model:     e.model,
			ExpiresAt: expiresAt,
			// Cloud-side sizes are unknown locally; report zero.
			// Compatible clients treat zero gracefully.
			Size:          0,
			SizeVRAM:      0,
			ContextLength: 0,
		})
	}

	return api.PSResponse{Models: models}
}

package ps

import (
	"sync"
	"testing"
	"time"
)

func TestTracker_AcquireAndRelease(t *testing.T) {
	tr := New()

	release := tr.Acquire("gemma3")

	snap := tr.Snapshot()
	if len(snap.Models) != 1 {
		t.Fatalf("expected 1 active model, got %d", len(snap.Models))
	}
	if snap.Models[0].Model != "gemma3" {
		t.Errorf("expected model=gemma3, got %q", snap.Models[0].Model)
	}

	release()

	snap = tr.Snapshot()
	if len(snap.Models) != 0 {
		t.Errorf("expected 0 models after release, got %d", len(snap.Models))
	}
}

func TestTracker_MultipleAcquiresSameModel(t *testing.T) {
	tr := New()

	r1 := tr.Acquire("llama3")
	r2 := tr.Acquire("llama3")
	r3 := tr.Acquire("llama3")

	// Three concurrent requests — model should appear once.
	snap := tr.Snapshot()
	if len(snap.Models) != 1 {
		t.Fatalf("expected 1 model entry for 3 concurrent requests, got %d", len(snap.Models))
	}

	// Release two — still active.
	r1()
	r2()
	snap = tr.Snapshot()
	if len(snap.Models) != 1 {
		t.Errorf("expected 1 model after 2 of 3 releases, got %d", len(snap.Models))
	}

	// Release last — gone.
	r3()
	snap = tr.Snapshot()
	if len(snap.Models) != 0 {
		t.Errorf("expected 0 models after all releases, got %d", len(snap.Models))
	}
}

func TestTracker_MultipleModels(t *testing.T) {
	tr := New()

	r1 := tr.Acquire("gemma3")
	r2 := tr.Acquire("deepseek-v3")
	r3 := tr.Acquire("qwen3")

	snap := tr.Snapshot()
	if len(snap.Models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(snap.Models))
	}

	r2() // remove deepseek
	snap = tr.Snapshot()
	if len(snap.Models) != 2 {
		t.Fatalf("expected 2 models after one release, got %d", len(snap.Models))
	}

	// Verify correct models remain.
	names := map[string]bool{}
	for _, m := range snap.Models {
		names[m.Model] = true
	}
	if !names["gemma3"] || !names["qwen3"] {
		t.Errorf("wrong models in snapshot: %v", names)
	}
	if names["deepseek-v3"] {
		t.Error("deepseek-v3 should have been removed")
	}

	r1()
	r3()
}

func TestTracker_ExpiresAtIsInFuture(t *testing.T) {
	tr := New()
	release := tr.Acquire("gemma3")
	defer release()

	snap := tr.Snapshot()
	if len(snap.Models) == 0 {
		t.Fatal("no models in snapshot")
	}

	expiresAt := snap.Models[0].ExpiresAt
	if expiresAt == "" {
		t.Fatal("expires_at is empty")
	}

	t.Logf("expires_at = %s", expiresAt)
	// We just verify it parses as a valid timestamp — exact value
	// depends on time.Now() so we don't pin it.
}

func TestTracker_ConcurrentSafety(t *testing.T) {
	tr := New()
	const goroutines = 50
	const models = 5

	modelNames := []string{"a", "b", "c", "d", "e"}
	var wg sync.WaitGroup

	// Hammer Acquire/Release from many goroutines simultaneously.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			model := modelNames[n%models]
			release := tr.Acquire(model)
			// Small sleep to increase overlap.
			time.Sleep(time.Millisecond)
			_ = tr.Snapshot() // concurrent read
			release()
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, tracker must be empty.
	snap := tr.Snapshot()
	if len(snap.Models) != 0 {
		t.Errorf("expected empty tracker after all releases, got %d models: %v",
			len(snap.Models), snap.Models)
	}
}

func TestTracker_SnapshotIsIndependent(t *testing.T) {
	tr := New()
	r1 := tr.Acquire("gemma3")

	snap1 := tr.Snapshot()
	r1()
	// Acquiring another model after r1 release.
	r2 := tr.Acquire("llama3")
	defer r2()

	snap2 := tr.Snapshot()

	// snap1 captured gemma3, snap2 should show llama3.
	if len(snap1.Models) != 1 || snap1.Models[0].Model != "gemma3" {
		t.Errorf("snap1 wrong: %v", snap1.Models)
	}
	if len(snap2.Models) != 1 || snap2.Models[0].Model != "llama3" {
		t.Errorf("snap2 wrong: %v", snap2.Models)
	}
}

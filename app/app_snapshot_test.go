package app

import (
	"encoding/json"
	"testing"

	storage "github.com/slidebolt/sb-storage-sdk"

	"github.com/slidebolt/plugin-zigbee/internal/hostapi"
)

// pruneSpyStore reports a pre-existing entity from an earlier snapshot and
// records every Delete so tests can assert nothing is pruned.
type pruneSpyStore struct {
	fakeStore
	existing []storage.Entry
	deleted  []string
}

func (p *pruneSpyStore) Search(string) ([]storage.Entry, error) { return p.existing, nil }

func (p *pruneSpyStore) Delete(k storage.Keyed) error {
	p.deleted = append(p.deleted, k.Key())
	return nil
}

// A device absent from a snapshot may merely be unreachable, asleep, or
// mid-rejoin. storage.Delete also removes the user-owned .profile.json
// sidecar, so pruning on absence would silently destroy labels/meta/profile.
// applySnapshot must be upsert-only.
func TestApplySnapshotDoesNotPruneAbsentDevices(t *testing.T) {
	store := &pruneSpyStore{
		existing: []storage.Entry{
			{Key: "plugin-zigbee.0xabsent.default-light", Data: json.RawMessage(`{}`)},
			{Key: "plugin-zigbee.0xabsent", Data: json.RawMessage(`{}`)},
		},
	}
	app := New()
	app.store = store

	// Snapshot contains a different device; 0xabsent is nowhere in it.
	snapshot := hostapi.Snapshot{
		Sequence: 1,
		Devices: []hostapi.Device{{
			ID: "0xpresent", Name: "Bulb", Kind: "router",
			Interview: "successful", Available: true,
			Components: []hostapi.Component{{
				ID: "default-light", Kind: "light", Name: "light", Revision: 1,
				Features: map[string]hostapi.Feature{"power": {Write: true}},
				State:    map[string]any{"power": "OFF"},
			}},
		}},
	}

	if err := app.applySnapshot(snapshot); err != nil {
		t.Fatalf("applySnapshot: %v", err)
	}

	if len(store.deleted) != 0 {
		t.Fatalf("applySnapshot pruned %d key(s) absent from the snapshot: %v; "+
			"Delete also removes the user-owned sidecar, so absence must never prune",
			len(store.deleted), store.deleted)
	}
	if _, ok := store.saved["plugin-zigbee.0xpresent.default-light"]; !ok {
		t.Fatalf("expected present device to be saved; saved keys: %v", keysOf(store.saved))
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

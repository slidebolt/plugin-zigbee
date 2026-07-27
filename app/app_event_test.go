package app

import (
	"encoding/json"
	"testing"

	domain "github.com/slidebolt/sb-domain"
	storage "github.com/slidebolt/sb-storage-sdk"

	"github.com/slidebolt/plugin-zigbee/internal/hostapi"
)

type fakeStore struct {
	saved map[string]json.RawMessage
}

func (f *fakeStore) Save(v storage.Keyed) error {
	if f.saved == nil {
		f.saved = map[string]json.RawMessage{}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f.saved[v.Key()] = data
	return nil
}
func (f *fakeStore) Get(storage.Keyed) (json.RawMessage, error) { return nil, nil }
func (f *fakeStore) Delete(storage.Keyed) error                 { return nil }
func (f *fakeStore) Search(string) ([]storage.Entry, error)     { return nil, nil }
func (f *fakeStore) SearchFiles(storage.StorageTarget, string) ([]storage.Entry, error) {
	return nil, nil
}
func (f *fakeStore) Query(storage.Query) ([]storage.Entry, error) { return nil, nil }
func (f *fakeStore) WriteFile(storage.StorageTarget, storage.Keyed, json.RawMessage) error {
	return nil
}
func (f *fakeStore) ReadFile(storage.StorageTarget, storage.Keyed) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeStore) DeleteFile(storage.StorageTarget, storage.Keyed) error { return nil }
func (f *fakeStore) SetPrivate(storage.Keyed, json.RawMessage) error       { return nil }
func (f *fakeStore) GetPrivate(storage.Keyed) (json.RawMessage, error)     { return nil, nil }
func (f *fakeStore) DeletePrivate(storage.Keyed) error                     { return nil }
func (f *fakeStore) SetInternal(storage.Keyed, json.RawMessage) error      { return nil }
func (f *fakeStore) GetInternal(storage.Keyed) (json.RawMessage, error)    { return nil, nil }
func (f *fakeStore) DeleteInternal(storage.Keyed) error                    { return nil }
func (f *fakeStore) SetProfile(storage.Keyed, json.RawMessage) error       { return nil }
func (f *fakeStore) Close()                                                {}

func TestComponentStateChangedSavesEntityAndDeviceAggregate(t *testing.T) {
	store := &fakeStore{}
	app := New()
	app.store = store
	app.devices["0x1"] = hostapi.Device{
		ID: "0x1", Name: "Bulb", Kind: "router", Interview: "successful", Available: true,
		Components: []hostapi.Component{{
			ID: "default-light", Kind: "light", Name: "light", Revision: 1,
			Features: map[string]hostapi.Feature{"power": {Write: true}, "brightness": {Write: true}},
			State:    map[string]any{"power": "OFF", "brightness": float64(1)},
		}},
	}
	data, err := json.Marshal(struct {
		DeviceID    string         `json:"deviceId"`
		ComponentID string         `json:"componentId"`
		Revision    int64          `json:"revision"`
		Patch       map[string]any `json:"patch"`
	}{DeviceID: "0x1", ComponentID: "default-light", Revision: 2, Patch: map[string]any{"power": "ON", "brightness": float64(80)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyEvent(hostapi.Event{Type: "component.state_changed", Data: data}); err != nil {
		t.Fatal(err)
	}

	entityData := store.saved["plugin-zigbee.0x1.default-light"]
	if len(entityData) == 0 {
		t.Fatalf("entity was not saved; keys=%v", store.saved)
	}
	var entity domain.Entity
	if err := json.Unmarshal(entityData, &entity); err != nil {
		t.Fatal(err)
	}
	light := entity.State.(domain.Light)
	if !light.Power || light.Brightness != 80 {
		t.Fatalf("entity light = %#v", light)
	}

	deviceData := store.saved["plugin-zigbee.0x1"]
	if len(deviceData) == 0 {
		t.Fatalf("device aggregate was not saved; keys=%v", store.saved)
	}
	var aggregate struct {
		Entities []domain.Entity `json:"entities"`
	}
	if err := json.Unmarshal(deviceData, &aggregate); err != nil {
		t.Fatal(err)
	}
	if len(aggregate.Entities) != 2 {
		t.Fatalf("aggregate entities = %d", len(aggregate.Entities))
	}
	embedded := aggregate.Entities[1].State.(domain.Light)
	if !embedded.Power || embedded.Brightness != 80 {
		t.Fatalf("aggregate light = %#v", embedded)
	}
}

type removalSpyStore struct {
	fakeStore
	deleted      []string
	profileSaved []string
}

func (s *removalSpyStore) Delete(k storage.Keyed) error {
	s.deleted = append(s.deleted, k.Key())
	return nil
}

func (s *removalSpyStore) SetProfile(k storage.Keyed, _ json.RawMessage) error {
	s.profileSaved = append(s.profileSaved, k.Key())
	return nil
}

func TestDeviceRemovedPersistsUnavailableWithoutDeleting(t *testing.T) {
	store := &removalSpyStore{}
	app := New()
	app.store = store
	app.devices["0x1"] = testLightDevice("0x1")

	if err := app.applyEvent(deviceRemovedEvent(t, "0x1")); err != nil {
		t.Fatal(err)
	}

	if len(store.deleted) != 0 {
		t.Fatalf("device.removed deleted %d key(s): %v; Delete removes user-owned profile sidecars", len(store.deleted), store.deleted)
	}
	if len(store.profileSaved) != 0 {
		t.Fatalf("device.removed wrote profile sidecars: %v", store.profileSaved)
	}

	entityData := store.saved["plugin-zigbee.0x1.availability"]
	if len(entityData) == 0 {
		t.Fatalf("availability entity was not saved; keys=%v", store.saved)
	}
	var entity domain.Entity
	if err := json.Unmarshal(entityData, &entity); err != nil {
		t.Fatal(err)
	}
	availability := entity.State.(domain.BinarySensor)
	if availability.On {
		t.Fatalf("availability after device.removed = %#v, want unavailable", availability)
	}

	deviceData := store.saved["plugin-zigbee.0x1"]
	if len(deviceData) == 0 {
		t.Fatalf("device aggregate was not saved; keys=%v", store.saved)
	}
	var aggregate struct {
		Entities []domain.Entity `json:"entities"`
	}
	if err := json.Unmarshal(deviceData, &aggregate); err != nil {
		t.Fatal(err)
	}
	if len(aggregate.Entities) == 0 {
		t.Fatal("device aggregate has no entities")
	}
	embeddedAvailability := aggregate.Entities[0].State.(domain.BinarySensor)
	if embeddedAvailability.On {
		t.Fatalf("embedded availability after device.removed = %#v, want unavailable", embeddedAvailability)
	}
}

func TestDeviceRemovedThenRediscoveredKeepsPluginWritesNonDestructive(t *testing.T) {
	store := &removalSpyStore{}
	app := New()
	app.store = store
	app.devices["0x1"] = testLightDevice("0x1")

	if err := app.applyEvent(deviceRemovedEvent(t, "0x1")); err != nil {
		t.Fatal(err)
	}

	rejoined := testLightDevice("0x1")
	rejoined.Name = "Rejoined Bulb"
	data, err := json.Marshal(rejoined)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyEvent(hostapi.Event{Type: "device.upserted", Data: data}); err != nil {
		t.Fatal(err)
	}

	if len(store.deleted) != 0 {
		t.Fatalf("remove/rejoin deleted %d key(s): %v", len(store.deleted), store.deleted)
	}
	if len(store.profileSaved) != 0 {
		t.Fatalf("remove/rejoin wrote profile sidecars: %v", store.profileSaved)
	}

	entityData := store.saved["plugin-zigbee.0x1.availability"]
	if len(entityData) == 0 {
		t.Fatalf("availability entity was not saved after rejoin; keys=%v", store.saved)
	}
	var entity domain.Entity
	if err := json.Unmarshal(entityData, &entity); err != nil {
		t.Fatal(err)
	}
	availability := entity.State.(domain.BinarySensor)
	if !availability.On {
		t.Fatalf("availability after rejoin = %#v, want available", availability)
	}
}

func testLightDevice(id string) hostapi.Device {
	return hostapi.Device{
		ID: id, Name: "Bulb", Kind: "router", Interview: "successful", Available: true,
		Reachability: hostapi.Reachability{Status: "reachable"},
		Components: []hostapi.Component{{
			ID: "default-light", Kind: "light", Name: "light", Revision: 1,
			Features: map[string]hostapi.Feature{"power": {Write: true}, "brightness": {Write: true}},
			State:    map[string]any{"power": "OFF", "brightness": float64(1)},
		}},
	}
}

func deviceRemovedEvent(t *testing.T, deviceID string) hostapi.Event {
	t.Helper()
	data, err := json.Marshal(struct {
		DeviceID string `json:"deviceId"`
	}{DeviceID: deviceID})
	if err != nil {
		t.Fatal(err)
	}
	return hostapi.Event{Type: "device.removed", Data: data}
}

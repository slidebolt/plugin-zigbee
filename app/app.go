package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	contract "github.com/slidebolt/sb-contract"
	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"

	"github.com/slidebolt/plugin-zigbee/internal/hostapi"
	"github.com/slidebolt/plugin-zigbee/internal/translate"
)

const PluginID = "plugin-zigbee"

type App struct {
	msg    messenger.Messenger
	store  storage.Storage
	cmds   *messenger.Commands
	subs   []messenger.Subscription
	host   *hostapi.Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.RWMutex
	snapshot hostapi.Snapshot
	devices  map[string]hostapi.Device
}

func New() *App {
	return &App{
		devices:  map[string]hostapi.Device{},
		snapshot: hostapi.Snapshot{Pairing: hostapi.Pairing{DurationSeconds: 60}},
	}
}

func (a *App) Hello() contract.HelloResponse {
	return contract.HelloResponse{ID: PluginID, Kind: contract.KindPlugin, ContractVersion: contract.ContractVersion, DependsOn: []string{"messenger", "storage"}}
}

func (a *App) OnStart(deps map[string]json.RawMessage) (json.RawMessage, error) {
	msg, err := messenger.Connect(deps)
	if err != nil {
		return nil, fmt.Errorf("connect messenger: %w", err)
	}
	a.msg = msg
	store, err := storage.Connect(deps)
	if err != nil {
		msg.Close()
		return nil, fmt.Errorf("connect storage: %w", err)
	}
	a.store = store
	host, err := hostapi.New(os.Getenv("ZIGBEE_HOST_URL"), os.Getenv("ZIGBEE_HOST_TOKEN"))
	if err != nil {
		store.Close()
		msg.Close()
		return nil, err
	}
	a.host = host
	a.cmds = messenger.NewCommands(msg, domain.LookupCommand)
	sub, err := a.cmds.ReceiveMessage(PluginID+".>", a.handleCommand)
	if err != nil {
		store.Close()
		msg.Close()
		return nil, err
	}
	a.subs = append(a.subs, sub)
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.wg.Add(2)
	go a.syncLoop()
	go a.countdownLoop()
	log.Printf("plugin-zigbee: started; host=%s", os.Getenv("ZIGBEE_HOST_URL"))
	return nil, nil
}

func (a *App) OnShutdown() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	for _, sub := range a.subs {
		_ = sub.Unsubscribe()
	}
	if a.store != nil {
		a.store.Close()
	}
	if a.msg != nil {
		a.msg.Close()
	}
	return nil
}

func (a *App) syncLoop() {
	defer a.wg.Done()
	backoff := time.Second
	for a.ctx.Err() == nil {
		snapshot, err := a.host.Snapshot(a.ctx)
		if err != nil {
			a.markHostOffline(err)
			if !wait(a.ctx, backoff) {
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		if err := a.applySnapshot(snapshot); err != nil {
			log.Printf("plugin-zigbee: apply snapshot: %v", err)
		}
		cursor := snapshot.Sequence
		err = a.host.Events(a.ctx, snapshot.StreamID, cursor, func(event hostapi.Event) error {
			if event.StreamID != "" && event.StreamID != snapshot.StreamID {
				return hostapi.ErrCursorExpired
			}
			if event.ID <= cursor {
				return nil
			}
			if err := a.applyEvent(event); err != nil {
				return err
			}
			cursor = event.ID
			return nil
		})
		if a.ctx.Err() != nil {
			return
		}
		if !errors.Is(err, hostapi.ErrCursorExpired) {
			a.markHostOffline(err)
		}
		if !errors.Is(err, hostapi.ErrCursorExpired) && !wait(a.ctx, backoff) {
			return
		}
	}
}

func (a *App) applySnapshot(snapshot hostapi.Snapshot) error {
	a.mu.Lock()
	a.snapshot = snapshot
	a.devices = make(map[string]hostapi.Device, len(snapshot.Devices))
	for _, device := range snapshot.Devices {
		a.devices[device.ID] = device
	}
	a.mu.Unlock()
	desired := map[string]struct{}{}
	coordinator, coordinatorEntities := translate.Coordinator(snapshot)
	coordinator.Entities = coordinatorEntities
	if err := a.store.Save(coordinator); err != nil {
		return err
	}
	desired[coordinator.Key()] = struct{}{}
	for _, entity := range coordinatorEntities {
		if err := a.store.Save(entity); err != nil {
			return err
		}
		desired[entity.Key()] = struct{}{}
	}
	for _, device := range snapshot.Devices {
		if err := a.saveDevice(device, desired); err != nil {
			return err
		}
	}
	entries, err := a.store.Search(PluginID + ".>")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, ok := desired[entry.Key]; ok {
			continue
		}
		if err := a.store.Delete(key(entry.Key)); err != nil {
			return err
		}
	}
	log.Printf("plugin-zigbee: synchronized %d device(s) at sequence %d", len(snapshot.Devices), snapshot.Sequence)
	return nil
}

func (a *App) saveDevice(device hostapi.Device, desired map[string]struct{}) error {
	dev := translate.Device(device)
	entities := translate.Entities(device)
	dev.Entities = publicEntities(entities)
	if err := a.store.Save(dev); err != nil {
		return err
	}
	if desired != nil {
		desired[dev.Key()] = struct{}{}
	}
	for _, entity := range entities {
		if err := a.store.Save(entity); err != nil {
			return err
		}
		if desired != nil {
			desired[entity.Key()] = struct{}{}
		}
	}
	return nil
}

func (a *App) applyEvent(event hostapi.Event) error {
	switch event.Type {
	case "device.upserted":
		var device hostapi.Device
		if err := json.Unmarshal(event.Data, &device); err != nil {
			return err
		}
		a.mu.Lock()
		a.devices[device.ID] = device
		a.mu.Unlock()
		return a.saveDevice(device, nil)
	case "device.removed":
		var data struct {
			DeviceID string `json:"deviceId"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.mu.Lock()
		delete(a.devices, data.DeviceID)
		a.mu.Unlock()
		entries, _ := a.store.Search(PluginID + "." + data.DeviceID + ".>")
		for _, entry := range entries {
			_ = a.store.Delete(key(entry.Key))
		}
		return a.store.Delete(domain.DeviceKey{Plugin: PluginID, ID: data.DeviceID})
	case "device.availability_changed":
		var data struct {
			DeviceID     string               `json:"deviceId"`
			Reachability hostapi.Reachability `json:"reachability"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.mu.Lock()
		device, ok := a.devices[data.DeviceID]
		if ok {
			device.Reachability = data.Reachability
			device.Available = data.Reachability.Status == "reachable"
			a.devices[data.DeviceID] = device
		}
		a.mu.Unlock()
		if ok {
			return a.saveDevice(device, nil)
		}
	case "component.state_changed":
		var data struct {
			DeviceID, ComponentID string
			Revision              int64
			Patch                 map[string]any
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.mu.Lock()
		device, ok := a.devices[data.DeviceID]
		if ok {
			for i := range device.Components {
				if device.Components[i].ID == data.ComponentID && data.Revision >= device.Components[i].Revision {
					for name, value := range data.Patch {
						device.Components[i].State[name] = value
					}
					device.Components[i].Revision = data.Revision
					entity := translate.Entity(data.DeviceID, device.Components[i])
					a.devices[data.DeviceID] = device
					a.mu.Unlock()
					return a.store.Save(entity)
				}
			}
		}
		a.mu.Unlock()
	case "coordinator.changed":
		var coordinator hostapi.Coordinator
		if err := json.Unmarshal(event.Data, &coordinator); err != nil {
			return err
		}
		a.mu.Lock()
		a.snapshot.Coordinator = coordinator
		snapshot := a.snapshot
		a.mu.Unlock()
		return a.saveCoordinator(snapshot)
	case "pairing.changed":
		var pairing hostapi.Pairing
		if err := json.Unmarshal(event.Data, &pairing); err != nil {
			return err
		}
		a.mu.Lock()
		a.snapshot.Pairing = pairing
		snapshot := a.snapshot
		a.mu.Unlock()
		return a.saveCoordinator(snapshot)
	}
	return nil
}

func (a *App) saveCoordinator(snapshot hostapi.Snapshot) error {
	device, entities := translate.Coordinator(snapshot)
	device.Entities = entities
	if err := a.store.Save(device); err != nil {
		return err
	}
	for _, entity := range entities {
		if err := a.store.Save(entity); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) markHostOffline(err error) {
	if err != nil {
		log.Printf("plugin-zigbee: host unavailable: %v", err)
	}
	a.mu.Lock()
	a.snapshot.Coordinator.Connected = false
	a.snapshot.Coordinator.State = "disconnected"
	a.snapshot.Coordinator.LastError = fmt.Sprint(err)
	snapshot := a.snapshot
	a.mu.Unlock()
	_ = a.saveCoordinator(snapshot)
}

func (a *App) countdownLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case now := <-ticker.C:
			a.mu.Lock()
			if a.snapshot.Pairing.EndsAt == "" {
				a.mu.Unlock()
				continue
			}
			end, err := time.Parse(time.RFC3339Nano, a.snapshot.Pairing.EndsAt)
			if err != nil {
				a.mu.Unlock()
				continue
			}
			delta := end.Sub(now)
			remaining := int((delta + time.Second - 1) / time.Second)
			if remaining < 0 {
				remaining = 0
			}
			if remaining == a.snapshot.Pairing.RemainingSeconds {
				a.mu.Unlock()
				continue
			}
			a.snapshot.Pairing.RemainingSeconds = remaining
			if remaining == 0 {
				a.snapshot.Pairing.Active = false
				a.snapshot.Pairing.EndsAt = ""
			}
			snapshot := a.snapshot
			a.mu.Unlock()
			_ = a.saveCoordinator(snapshot)
		}
	}
}

func (a *App) handleCommand(addr messenger.Address, cmd any, message *messenger.Message) {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		if addr.DeviceID == "coordinator" {
			var err error
			switch c := cmd.(type) {
			case domain.SwitchTurnOn:
				if addr.EntityID == "permit-join" {
					err = a.host.SetPairing(ctx, true)
				}
			case domain.SwitchTurnOff:
				if addr.EntityID == "permit-join" {
					err = a.host.SetPairing(ctx, false)
				}
			case domain.NumberSetValue:
				if addr.EntityID == "pairing-duration" {
					err = a.host.ConfigurePairing(ctx, int(c.Value))
				}
			default:
				err = fmt.Errorf("unsupported coordinator command %T", cmd)
			}
			if err != nil {
				log.Printf("plugin-zigbee: coordinator command failed: %v", err)
			}
			return
		}
		raw, err := a.store.GetPrivate(domain.EntityKey{Plugin: PluginID, DeviceID: addr.DeviceID, ID: addr.EntityID})
		if err != nil {
			log.Printf("plugin-zigbee: command route missing for %s: %v", addr.Key(), err)
			return
		}
		var route translate.Route
		if err := json.Unmarshal(raw, &route); err != nil {
			log.Printf("plugin-zigbee: invalid command route for %s: %v", addr.Key(), err)
			return
		}
		action, parameters, err := translate.Command(cmd)
		if err != nil {
			log.Printf("plugin-zigbee: %v", err)
			return
		}
		traceID := messenger.TraceID(message.Headers)
		response, err := a.host.Command(ctx, route.DeviceID, route.ComponentID, hostapi.CommandRequest{Action: action, Parameters: parameters, RequestID: traceID, ObservationTimeoutMS: 30_000})
		if err != nil {
			log.Printf("plugin-zigbee: command %s failed: %v", addr.Key(), err)
			return
		}
		log.Printf("plugin-zigbee: command %s queued as %s (%s)", addr.Key(), response.Command.ID, response.Command.Status)
	}()
}

type key string

func (k key) Key() string { return string(k) }

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func publicEntities(entities []domain.Entity) []domain.Entity {
	result := make([]domain.Entity, len(entities))
	copy(result, entities)
	for i := range result {
		result[i].Private = nil
	}
	return result
}

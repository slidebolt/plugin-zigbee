package translate

import (
	"reflect"
	"testing"

	domain "github.com/slidebolt/sb-domain"

	"github.com/slidebolt/plugin-zigbee/internal/hostapi"
)

func boolPtr(v float64) *float64 { return &v }

func TestLightTranslation(t *testing.T) {
	component := hostapi.Component{
		ID: "default-light", Kind: "light", Name: "light",
		Features: map[string]hostapi.Feature{
			"power": {Write: true}, "brightness": {Write: true, Minimum: boolPtr(0), Maximum: boolPtr(254)},
		},
		State: map[string]any{"power": "ON", "brightness": float64(40)},
	}
	entity := Entity("0x1", component)
	light, ok := entity.State.(domain.Light)
	if !ok || !light.Power || light.Brightness != 40 {
		t.Fatalf("unexpected light: %#v", entity.State)
	}
	if len(entity.Commands) != 3 {
		t.Fatalf("commands: %v", entity.Commands)
	}
	action, params, err := Command(domain.LightSetBrightness{Brightness: 12})
	if err != nil || action != "set_brightness" || params["brightness"] != 12 {
		t.Fatalf("command: %s %#v %v", action, params, err)
	}
}

func TestLightTranslationColorState(t *testing.T) {
	component := hostapi.Component{
		ID: "default-light", Kind: "light", Name: "light",
		Features: map[string]hostapi.Feature{
			"power": {Write: true}, "brightness": {Write: true}, "color": {Write: true}, "color_temp": {Write: true},
		},
		State: map[string]any{"power": "ON", "brightness": float64(40), "color": map[string]any{"x": 0.312, "y": 0.329}},
	}
	entity := Entity("0x1", component)
	light, ok := entity.State.(domain.Light)
	if !ok {
		t.Fatalf("state type = %T", entity.State)
	}
	if light.ColorMode != "xy" || !reflect.DeepEqual(light.XY, []float64{0.312, 0.329}) {
		t.Fatalf("light color = %#v", light)
	}

	component.State = map[string]any{"power": "ON", "color_mode": "rgb", "color": map[string]any{"r": float64(255), "g": float64(128), "b": float64(64)}}
	entity = Entity("0x1", component)
	light = entity.State.(domain.Light)
	if light.ColorMode != "rgb" || !reflect.DeepEqual(light.RGB, []int{255, 128, 64}) {
		t.Fatalf("light rgb = %#v", light)
	}
}

func TestCoordinatorEntities(t *testing.T) {
	snapshot := hostapi.Snapshot{Coordinator: hostapi.Coordinator{Connected: true, Channel: 20, PairedDevices: 2}, Pairing: hostapi.Pairing{DurationSeconds: 60}}
	_, entities := Coordinator(snapshot)
	if len(entities) != 7 {
		t.Fatalf("entities=%d", len(entities))
	}
	number, ok := entities[2].State.(domain.Number)
	if !ok || number.Min != 10 || number.Max != 254 {
		t.Fatalf("duration=%#v", entities[2].State)
	}
	if entities[3].ID != "refresh-network-diagnostics" || !reflect.DeepEqual(entities[3].Commands, []string{"button_press"}) || entities[3].SecurityGroup != "system" {
		t.Fatalf("refresh entity=%#v", entities[3])
	}
}

func TestDeviceDiagnosticsUseGenericDomainEntities(t *testing.T) {
	lqi, previous, depth := 147.0, 25760, 2
	device := hostapi.Device{ID: "0xb0ce18140017459c", Name: "Bulb 02", Available: true,
		Reachability: hostapi.Reachability{Status: "degraded", LastSeen: "2026-11-17T20:00:00Z", Reason: "device_unreachable"},
		Network:      &hostapi.NetworkObservation{Role: "router", Address: 26510, PreviousAddress: &previous, ParentID: "0x00124b0000000001", Relationship: "child", Depth: &depth, LinkQuality: &lqi, LastInboundAt: "2026-11-17T20:00:00Z"},
	}
	translated := Device(device)
	if translated.Status == nil || translated.Status.Health != "degraded" {
		t.Fatalf("device=%#v", translated)
	}
	entities := Entities(device)
	groups, err := domain.DeviceSecurityGroups(translated, entities...)
	if err != nil || !reflect.DeepEqual(groups, []string{"system"}) {
		t.Fatalf("security groups=%v err=%v", groups, err)
	}
	if len(entities) != 3 {
		t.Fatalf("entities=%#v", entities)
	}
	if entities[1].Type != "sensor" || entities[1].SecurityGroup != "system" {
		t.Fatalf("signal=%#v", entities[1])
	}
	diagnostic, ok := entities[2].State.(domain.Diagnostic)
	if !ok || diagnostic["parentDeviceKey"] != "plugin-zigbee.0x00124b0000000001" || diagnostic["address"] != 26510 {
		t.Fatalf("diagnostic=%#v", entities[2].State)
	}
}

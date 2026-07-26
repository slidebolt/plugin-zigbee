package translate

import (
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

func TestCoordinatorEntities(t *testing.T) {
	snapshot := hostapi.Snapshot{Coordinator: hostapi.Coordinator{Connected: true, Channel: 20, PairedDevices: 2}, Pairing: hostapi.Pairing{DurationSeconds: 60}}
	_, entities := Coordinator(snapshot)
	if len(entities) != 6 {
		t.Fatalf("entities=%d", len(entities))
	}
	number, ok := entities[2].State.(domain.Number)
	if !ok || number.Min != 10 || number.Max != 254 {
		t.Fatalf("duration=%#v", entities[2].State)
	}
}

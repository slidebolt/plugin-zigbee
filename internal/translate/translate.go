package translate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	domain "github.com/slidebolt/sb-domain"

	"github.com/slidebolt/plugin-zigbee/internal/hostapi"
)

const PluginID = "plugin-zigbee"

type Route struct {
	DeviceID    string `json:"deviceId"`
	ComponentID string `json:"componentId"`
	Kind        string `json:"kind"`
}

func Device(device hostapi.Device) domain.Device {
	meta := map[string]json.RawMessage{}
	putMeta(meta, "manufacturer", device.Manufacturer)
	putMeta(meta, "model", device.Model)
	putMeta(meta, "description", device.Description)
	putMeta(meta, "interview", device.Interview)
	return domain.Device{ID: device.ID, Plugin: PluginID, Name: device.Name, Meta: meta}
}

func Entities(device hostapi.Device) []domain.Entity {
	result := make([]domain.Entity, 0, len(device.Components)+1)
	result = append(result, domain.Entity{
		ID: "availability", Plugin: PluginID, DeviceID: device.ID, Type: "binary_sensor", Name: "Connectivity",
		State: domain.BinarySensor{On: device.Available, DeviceClass: "connectivity"},
	})
	for _, component := range device.Components {
		result = append(result, Entity(device.ID, component))
	}
	return result
}

func Entity(deviceID string, component hostapi.Component) domain.Entity {
	private, _ := json.Marshal(Route{DeviceID: deviceID, ComponentID: component.ID, Kind: component.Kind})
	return domain.Entity{
		ID: component.ID, Plugin: PluginID, DeviceID: deviceID, Type: component.Kind, Name: title(component.Name),
		Commands: commands(component), State: state(component), Private: private,
	}
}

func Coordinator(snapshot hostapi.Snapshot) (domain.Device, []domain.Entity) {
	device := domain.Device{ID: "coordinator", Plugin: PluginID, Name: "Zigbee Coordinator"}
	entities := []domain.Entity{
		{ID: "connection", Plugin: PluginID, DeviceID: device.ID, Type: "binary_sensor", Name: "Connection", State: domain.BinarySensor{On: snapshot.Coordinator.Connected, DeviceClass: "connectivity"}},
		{ID: "permit-join", Plugin: PluginID, DeviceID: device.ID, Type: "switch", Name: "Permit Join", Commands: []string{"switch_turn_on", "switch_turn_off"}, State: domain.Switch{Power: snapshot.Pairing.Active}},
		{ID: "pairing-duration", Plugin: PluginID, DeviceID: device.ID, Type: "number", Name: "Pairing Duration", Commands: []string{"number_set_value"}, State: domain.Number{Value: float64(snapshot.Pairing.DurationSeconds), Min: 10, Max: 254, Step: 1, Unit: "s"}},
		{ID: "pairing-remaining", Plugin: PluginID, DeviceID: device.ID, Type: "sensor", Name: "Pairing Remaining", State: domain.Sensor{Value: snapshot.Pairing.RemainingSeconds, Unit: "s"}},
		{ID: "paired-devices", Plugin: PluginID, DeviceID: device.ID, Type: "sensor", Name: "Paired Devices", State: domain.Sensor{Value: snapshot.Coordinator.PairedDevices}},
		{ID: "channel", Plugin: PluginID, DeviceID: device.ID, Type: "sensor", Name: "Channel", State: domain.Sensor{Value: snapshot.Coordinator.Channel}},
	}
	return device, entities
}

func Command(cmd any) (string, map[string]any, error) {
	switch c := cmd.(type) {
	case domain.LightTurnOn:
		return "turn_on", nil, nil
	case domain.LightTurnOff:
		return "turn_off", nil, nil
	case domain.LightSetBrightness:
		return "set_brightness", map[string]any{"brightness": c.Brightness}, nil
	case domain.LightSetColorTemp:
		return "set_color_temp", map[string]any{"temperature": c.Mireds, "brightness": c.Brightness}, nil
	case domain.LightSetRGB:
		return "set_rgb", map[string]any{"r": c.R, "g": c.G, "b": c.B, "brightness": c.Brightness}, nil
	case domain.LightSetHS:
		return "set_hs", map[string]any{"hue": c.Hue, "saturation": c.Saturation, "brightness": c.Brightness}, nil
	case domain.LightSetXY:
		return "set_xy", map[string]any{"x": c.X, "y": c.Y, "brightness": c.Brightness}, nil
	case domain.LightSetEffect:
		return "set_effect", map[string]any{"effect": c.Effect}, nil
	case domain.SwitchTurnOn:
		return "turn_on", nil, nil
	case domain.SwitchTurnOff:
		return "turn_off", nil, nil
	case domain.SwitchToggle:
		return "toggle", nil, nil
	case domain.FanTurnOn:
		return "turn_on", nil, nil
	case domain.FanTurnOff:
		return "turn_off", nil, nil
	case domain.FanSetSpeed:
		return "set_speed", map[string]any{"percentage": c.Percentage}, nil
	case domain.CoverOpen:
		return "set_position", map[string]any{"position": 100}, nil
	case domain.CoverClose:
		return "set_position", map[string]any{"position": 0}, nil
	case domain.CoverSetPosition:
		return "set_position", map[string]any{"position": c.Position}, nil
	case domain.LockLock:
		return "lock", nil, nil
	case domain.LockUnlock:
		return "unlock", nil, nil
	case domain.ButtonPress:
		return "press", nil, nil
	case domain.NumberSetValue:
		return "set_value", map[string]any{"value": c.Value}, nil
	case domain.SelectOption:
		return "select_option", map[string]any{"option": c.Option}, nil
	case domain.TextSetValue:
		return "set_value", map[string]any{"value": c.Value}, nil
	case domain.ClimateSetMode:
		return "set_hvac_mode", map[string]any{"mode": c.HVACMode}, nil
	case domain.ClimateSetTemperature:
		return "set_temperature", map[string]any{"temperature": c.Temperature}, nil
	default:
		return "", nil, fmt.Errorf("unsupported Zigbee command %T", cmd)
	}
}

func commands(component hostapi.Component) []string {
	has := func(name string) bool { return component.Features[name].Write }
	var result []string
	switch component.Kind {
	case "light":
		if has("power") {
			result = append(result, "light_turn_on", "light_turn_off")
		}
		if has("brightness") {
			result = append(result, "light_set_brightness")
		}
		if has("color_temp") {
			result = append(result, "light_set_color_temp")
		}
		if has("color_xy") || has("color") {
			result = append(result, "light_set_rgb", "light_set_hs", "light_set_xy")
		}
		if has("effect") {
			result = append(result, "light_set_effect")
		}
	case "switch":
		result = []string{"switch_turn_on", "switch_turn_off", "switch_toggle"}
	case "fan":
		result = []string{"fan_turn_on", "fan_turn_off", "fan_set_speed"}
	case "cover":
		result = []string{"cover_open", "cover_close", "cover_set_position"}
	case "lock":
		result = []string{"lock_lock", "lock_unlock"}
	case "button":
		result = []string{"button_press"}
	case "number":
		result = []string{"number_set_value"}
	case "select":
		result = []string{"select_option"}
	case "text":
		result = []string{"text_set_value"}
	case "climate":
		result = []string{"climate_set_mode", "climate_set_temperature"}
	}
	return result
}

func state(component hostapi.Component) any {
	s := component.State
	switch component.Kind {
	case "light":
		return domain.Light{Power: boolean(s["power"]), Brightness: integer(s["brightness"]), Temperature: integer(s["color_temp"]), Effect: stringValue(s["effect"])}
	case "switch":
		value, ok := s["power"]
		if !ok {
			value = first(s)
		}
		return domain.Switch{Power: boolean(value)}
	case "binary_sensor":
		return domain.BinarySensor{On: boolean(first(s))}
	case "sensor":
		name, value := firstEntry(s)
		return domain.Sensor{Value: value, Unit: component.Features[name].Unit}
	case "number":
		name, value := firstEntry(s)
		f := component.Features[name]
		return domain.Number{Value: number(value), Min: deref(f.Minimum), Max: deref(f.Maximum), Step: defaultStep(f.Step), Unit: f.Unit}
	case "select":
		name, value := firstEntry(s)
		return domain.Select{Option: stringValue(value), Options: stringsOf(component.Features[name].Values)}
	case "text":
		return domain.Text{Value: stringValue(first(s))}
	case "fan":
		return domain.Fan{Power: boolean(s["power"]), Percentage: integer(s["percentage"])}
	case "cover":
		return domain.Cover{Position: integer(s["position"])}
	case "lock":
		return domain.Lock{Locked: boolean(s["locked"])}
	case "button":
		return domain.Button{}
	case "climate":
		modes := stringsOf(component.Features["hvac_mode"].Values)
		return domain.Climate{HVACMode: stringValue(s["hvac_mode"]), HVACModes: modes, Temperature: number(s["temperature"]), CurrentTemperature: number(s["current_temperature"]), TemperatureUnit: "°C"}
	default:
		return s
	}
}

func boolean(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "ON" || v == "LOCK" || strings.EqualFold(v, "true")
	case float64:
		return v != 0
	}
	return false
}
func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return 0
}
func integer(value any) int { return int(number(value)) }
func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func first(values map[string]any) any { _, value := firstEntry(values); return value }
func firstEntry(values map[string]any) (string, any) {
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0], values[keys[0]]
	}
	return "", nil
}
func stringsOf(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, fmt.Sprint(value))
	}
	return result
}
func deref(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
func defaultStep(value *float64) float64 {
	if value == nil || *value == 0 {
		return 1
	}
	return *value
}
func title(value string) string {
	if value == "" {
		return "Zigbee Entity"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
func putMeta(meta map[string]json.RawMessage, key, value string) {
	if value != "" {
		data, _ := json.Marshal(value)
		meta[key] = data
	}
}

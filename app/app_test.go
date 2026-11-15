package app

import "testing"

func TestManifest(t *testing.T) {
	h := New().Hello()
	if h.ID != PluginID || len(h.DependsOn) != 2 || h.DependsOn[0] != "messenger" || h.DependsOn[1] != "storage" {
		t.Fatalf("manifest=%#v", h)
	}
	if err := h.Setup.Validate(); err != nil {
		t.Fatalf("setup definition: %v", err)
	}
	if h.Setup.Mode != "selectable" || len(h.Setup.Fields) != 2 {
		t.Fatalf("setup=%#v", h.Setup)
	}
	if h.Setup.Fields[0].Environment != "ZIGBEE_HOST_URL" || h.Setup.Fields[1].Environment != "ZIGBEE_HOST_TOKEN" {
		t.Fatalf("setup fields=%#v", h.Setup.Fields)
	}
}

package app

import "testing"

func TestManifest(t *testing.T) {
	h := New().Hello()
	if h.ID != PluginID || len(h.DependsOn) != 2 || h.DependsOn[0] != "messenger" || h.DependsOn[1] != "storage" {
		t.Fatalf("manifest=%#v", h)
	}
}

package app

import (
	"errors"
	"testing"

	runtime "github.com/slidebolt/sb-runtime"
)

type resourceRecorder struct{ states []string }

func (r *resourceRecorder) Ready(string)                    { r.states = append(r.states, "ready") }
func (r *resourceRecorder) Degraded(_, _, _ string, _ bool) { r.states = append(r.states, "degraded") }

var _ runtime.ResourceReporter = (*resourceRecorder)(nil)

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
	if len(h.Setup.Resources) != 1 || h.Setup.Resources[0].ID != zigbeeHostResource {
		t.Fatalf("resources=%#v", h.Setup.Resources)
	}
	if h.Setup.Fields[0].Environment != "ZIGBEE_HOST_URL" || h.Setup.Fields[1].Environment != "ZIGBEE_HOST_TOKEN" {
		t.Fatalf("setup fields=%#v", h.Setup.Fields)
	}
}

func TestResourceTransitions(t *testing.T) {
	recorder := &resourceRecorder{}
	a := New()
	a.SetResourceReporter(recorder)
	a.resourceReady()
	a.resourceDegraded(errors.New("offline"))
	if len(recorder.states) != 2 || recorder.states[0] != "ready" || recorder.states[1] != "degraded" {
		t.Fatalf("states=%v", recorder.states)
	}
}

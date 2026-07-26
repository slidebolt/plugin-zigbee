package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSnapshotAndEventStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/snapshot":
			_ = json.NewEncoder(w).Encode(Snapshot{APIVersion: 1, StreamID: "stream-1", Sequence: 4})
		case "/v1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "id: 5\nevent: pairing.changed\ndata: {\"version\":1,\"id\":5,\"streamId\":\"stream-1\",\"type\":\"pairing.changed\",\"data\":{\"active\":true}}\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil || snapshot.Sequence != 4 {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
	var received Event
	err = client.Events(context.Background(), snapshot.StreamID, snapshot.Sequence, func(event Event) error { received = event; return context.Canceled })
	if err != context.Canceled || received.ID != 5 {
		t.Fatalf("event=%#v err=%v", received, err)
	}
}

func TestCursorExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusConflict) }))
	defer server.Close()
	client, _ := New(server.URL, "secret")
	err := client.Events(context.Background(), "old", 1, func(Event) error { return nil })
	if err != ErrCursorExpired {
		t.Fatalf("err=%v", err)
	}
}

func TestEventStreamHasNoWholeResponseTimeout(t *testing.T) {
	client, err := New("http://127.0.0.1:47831", "token")
	if err != nil {
		t.Fatal(err)
	}
	if client.http.Timeout == 0 || client.events.Timeout != 0 {
		t.Fatalf("request timeout=%v event timeout=%v", client.http.Timeout, client.events.Timeout)
	}
}

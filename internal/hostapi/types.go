package hostapi

import "encoding/json"

type Feature struct {
	Read    bool     `json:"read"`
	Write   bool     `json:"write"`
	Reports bool     `json:"reports"`
	Unit    string   `json:"unit,omitempty"`
	Values  []any    `json:"values,omitempty"`
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	Step    *float64 `json:"step,omitempty"`
}

type Component struct {
	ID       string             `json:"id"`
	Kind     string             `json:"kind"`
	Name     string             `json:"name"`
	Endpoint string             `json:"endpoint,omitempty"`
	Revision int64              `json:"revision"`
	Features map[string]Feature `json:"features"`
	State    map[string]any     `json:"state"`
}

type Reachability struct {
	Status      string `json:"status"`
	LastSeen    string `json:"lastSeen,omitempty"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type Device struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Manufacturer string       `json:"manufacturer,omitempty"`
	Model        string       `json:"model,omitempty"`
	Description  string       `json:"description,omitempty"`
	Kind         string       `json:"kind"`
	Interview    string       `json:"interview"`
	Available    bool         `json:"available"`
	Reachability Reachability `json:"reachability"`
	Components   []Component  `json:"components"`
}

type Coordinator struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	Connected     bool   `json:"connected"`
	Channel       int    `json:"channel,omitempty"`
	PairedDevices int    `json:"pairedDevices"`
	LastError     string `json:"lastError,omitempty"`
}

type Pairing struct {
	Active           bool   `json:"active"`
	DurationSeconds  int    `json:"durationSeconds"`
	RemainingSeconds int    `json:"remainingSeconds"`
	EndsAt           string `json:"endsAt,omitempty"`
	Phase            string `json:"phase,omitempty"`
}

type Snapshot struct {
	APIVersion  int         `json:"apiVersion"`
	StreamID    string      `json:"streamId"`
	Sequence    int64       `json:"sequence"`
	Coordinator Coordinator `json:"coordinator"`
	Pairing     Pairing     `json:"pairing"`
	Devices     []Device    `json:"devices"`
}

type Event struct {
	Version   int             `json:"version"`
	ID        int64           `json:"id"`
	StreamID  string          `json:"streamId"`
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Data      json.RawMessage `json:"data"`
}

type CommandRequest struct {
	Action               string         `json:"action"`
	Parameters           map[string]any `json:"parameters,omitempty"`
	RequestID            string         `json:"requestId,omitempty"`
	ObservationTimeoutMS int            `json:"observationTimeoutMs,omitempty"`
	ObservationMode      string         `json:"observationMode,omitempty"`
}

type CommandResponse struct {
	Command struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"command"`
}

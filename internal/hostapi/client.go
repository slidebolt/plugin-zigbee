package hostapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrCursorExpired = errors.New("zigbee host event cursor expired")

type Client struct {
	base   string
	token  string
	http   *http.Client
	events *http.Client
}

func New(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("ZIGBEE_HOST_URL is required")
	}
	if token == "" {
		return nil, errors.New("ZIGBEE_HOST_TOKEN is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid Zigbee host URL: %w", err)
	}
	return &Client{
		base: baseURL, token: token,
		http:   &http.Client{Timeout: 15 * time.Second},
		events: &http.Client{},
	}, nil
}

func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := c.do(ctx, http.MethodGet, "/v1/snapshot", nil, &result)
	return result, err
}

func (c *Client) Command(ctx context.Context, deviceID, componentID string, command CommandRequest) (CommandResponse, error) {
	var result CommandResponse
	path := "/v1/devices/" + url.PathEscape(deviceID) + "/components/" + url.PathEscape(componentID) + "/commands"
	err := c.do(ctx, http.MethodPost, path, command, &result)
	return result, err
}

func (c *Client) ConfigurePairing(ctx context.Context, seconds int) error {
	return c.do(ctx, http.MethodPatch, "/v1/pairing/config", map[string]any{"durationSeconds": seconds}, nil)
}

func (c *Client) SetPairing(ctx context.Context, active bool) error {
	path := "/v1/pairing/stop"
	if active {
		path = "/v1/pairing/start"
	}
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

func (c *Client) Events(ctx context.Context, streamID string, after int64, receive func(Event) error) error {
	path := fmt.Sprintf("/v1/events?streamId=%s&after=%d", url.QueryEscape(streamID), after)
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Last-Event-ID", fmt.Sprint(after))
	resp, err := c.events.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ErrCursorExpired
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() > 0 {
				var event Event
				if err := json.Unmarshal([]byte(data.String()), &event); err != nil {
					return fmt.Errorf("decode event: %w", err)
				}
				if err := receive(event); err != nil {
					return err
				}
				data.Reset()
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func (c *Client) do(ctx context.Context, method, path string, payload any, target any) error {
	req, err := c.request(ctx, method, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var problem struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(data, &problem)
	if problem.Detail == "" {
		problem.Detail = strings.TrimSpace(string(data))
	}
	return fmt.Errorf("zigbee host HTTP %d: %s", resp.StatusCode, problem.Detail)
}

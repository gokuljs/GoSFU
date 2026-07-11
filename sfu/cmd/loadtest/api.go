package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
)

type apiClient struct {
	baseURL    string
	httpClient *http.Client
}

func newAPIClient(baseURL string) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type createRoomResponse struct {
	RoomID string `json:"roomId"`
}

type joinRoomRequest struct {
	Sdp          webrtc.SessionDescription `json:"sdp"`
	SystemPrompt string                    `json:"systemPrompt,omitempty"`
}

type joinRoomResponse struct {
	Sdp           webrtc.SessionDescription `json:"sdp"`
	ParticipantId string                    `json:"participantId"`
	RoomId        string                    `json:"roomId"`
}

type iceConfigResponse struct {
	ICEServers []webrtc.ICEServer `json:"iceServers"`
}

func (c *apiClient) createRoom() (string, error) {
	res, err := c.post("/room/create", nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", statusError("create room", res)
	}

	var body createRoomResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.RoomID == "" {
		return "", fmt.Errorf("create room: empty roomId")
	}
	return body.RoomID, nil
}

func (c *apiClient) iceConfig() ([]webrtc.ICEServer, error) {
	res, err := c.get("/ice-config")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, statusError("ice-config", res)
	}

	var body iceConfigResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	if len(body.ICEServers) == 0 {
		return nil, fmt.Errorf("ice-config: no servers returned")
	}
	return body.ICEServers, nil
}

func (c *apiClient) joinRoom(roomID string, offer webrtc.SessionDescription, systemPrompt string) (*joinRoomResponse, error) {
	reqBody := joinRoomRequest{
		Sdp:          offer,
		SystemPrompt: strings.TrimSpace(systemPrompt),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	res, err := c.post("/room/"+roomID+"/join", payload)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, statusError("join room", res)
	}

	var body joinRoomResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	return &body, nil
}

func (c *apiClient) deleteRoom(roomID string) error {
	res, err := c.delete("/room/" + roomID)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		return statusError("delete room", res)
	}
	return nil
}

func (c *apiClient) get(path string) (*http.Response, error) {
	return c.httpClient.Get(c.baseURL + path)
}

func (c *apiClient) post(path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *apiClient) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

func statusError(action string, res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("%s: HTTP %d", action, res.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", action, res.StatusCode, msg)
}

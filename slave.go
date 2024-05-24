package forjitree

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type SlaveHandlerFunc = func(*Slave, map[string]any)

type Slave struct {
	actionTaskUrl string
	handler       SlaveHandlerFunc
	running       bool
	client        *http.Client
}

func NewSlave() *Slave {
	return &Slave{
		client:  &http.Client{},
		running: true,
	}
}

func (s *Slave) Start(actionTaskUrl string, handler SlaveHandlerFunc) {
	s.actionTaskUrl = actionTaskUrl
	s.handler = handler
	s.running = true
	go s.run()
}

func (s *Slave) Stop() {
	s.running = false
}

func (s *Slave) run() {
	for {
		if !s.running {
			break
		}
		time.Sleep(time.Millisecond)

		readyResponse, err := s.makeRequest("slave=ready", nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if taskAny := readyResponse["task"]; taskAny != nil {
			if task, ok := taskAny.(map[string]any); ok {
				taskId := task["taskId"].(string)

				s.handler(s, task)

				s.makeRequest("slave=done&taskId="+taskId, task)
			}
		}
	}
}

func (s *Slave) makeRequest(params string, payload map[string]any) (map[string]any, error) {
	var jsonData []byte
	var err error

	if payload == nil {
		payload = map[string]any{}
	}
	jsonData, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", s.actionTaskUrl+"?"+params, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	readyResponse, err := s.client.Do(req)
	if err != nil || readyResponse.StatusCode != 200 {
		return nil, err
	}
	defer readyResponse.Body.Close()

	bodyBytes, err := io.ReadAll(readyResponse.Body)
	if err != nil {
		return nil, err
	}

	var body map[string]any
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Slave) Progress(task map[string]any) {
	s.makeRequest("slave=progress", task)
}

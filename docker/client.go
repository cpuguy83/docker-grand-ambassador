package docker

import (
	"encoding/json"
	"fmt"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
)

type (
	Docker interface {
		FetchAllContainers() ([]*Container, error)
		FetchContainer(name string) (*Container, error)
		GetEvents() chan *Event
	}

	Event struct {
		ContainerID string `json:"id"`
		Status      string `json:"status"`
	}

	Binding struct {
		HostIp   string
		HostPort string
	}

	NetworkSettings struct {
		IpAddress string
		Ports     map[string][]Binding
	}

	State struct {
		Running bool
	}

	Container struct {
		Id              string
		Name            string
		NetworkSettings *NetworkSettings
		State           State
	}

	dockerClient struct {
		path string
	}
)

func NewClient(path string) (Docker, error) {
	return &dockerClient{path}, nil
}

func (d *dockerClient) newConn() (*httputil.ClientConn, error) {
	proto, path := utils.SplitURI(d.path)
	conn, err := net.Dial(proto, path)

	if err != nil {
		return nil, err
	}
	return httputil.NewClientConn(conn, nil), nil
}

func (docker *dockerClient) FetchContainer(name string) (*Container, error) {
	var (
		method = "GET"
		uri    = fmt.Sprintf("/containers/%s/json", name)
	)
	req, err := http.NewRequest(method, fmt.Sprintf(uri), nil)
	if err != nil {
		return nil, err
	}

	c, err := docker.newConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(fmt.Sprintf("Request failed, status: %s", resp.StatusCode))
	}

	var container *Container
	err = json.NewDecoder(resp.Body).Decode(&container)
	if err != nil {
		return nil, err
	}
	return container, nil
}

func (docker *dockerClient) FetchAllContainers() ([]*Container, error) {
	var (
		method = "GET"
		uri    = "/containers/json"
	)
	req, err := http.NewRequest(method, fmt.Sprintf(uri), nil)
	if err != nil {
		return nil, err
	}

	c, err := docker.newConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(fmt.Sprintf("Request failed, status: %s", resp.StatusCode))
	}

	var containers []*Container
	if err = json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func (docker *dockerClient) newRequest(method, uri string) (io.ReadCloser, error) {
	req, err := http.NewRequest(method, fmt.Sprintf(uri), nil)
	if err != nil {
		return nil, err
	}

	c, err := docker.newConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}
	return nil, fmt.Errorf("invalid HTTP request %d %s", resp.StatusCode, resp.Status)
}

func (docker *dockerClient) GetEvents() chan *Event {
	eventChan := make(chan *Event, 100)
	go docker.getEvents(eventChan)
	return eventChan
}

func (docker *dockerClient) getEvents(eventChan chan *Event) {
	defer close(eventChan)

	resp, err := docker.newRequest("GET", "/events")
	if err != nil {
		return
	}

	dec := json.NewDecoder(resp)
	for {
		var event *Event
		if err := dec.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		eventChan <- event
	}
}

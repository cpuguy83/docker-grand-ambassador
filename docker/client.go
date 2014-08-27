package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"

	"github.com/cpuguy83/docker-grand-ambassador/utils"
)

type (
	Docker interface {
		FetchAllContainers() ([]*Container, error)
		FetchContainer(name string) (*Container, error)
		GetEvents() chan *Event
		Info() (*DaemonInfo, error)
		PullImage(name string) error
		CreateContainer(container map[string]interface{}) (string, error)
		StartContainer(string, interface{}) error
		RunContainer(map[string]interface{}) (string, error)
		RemoveContainer(name string, force, volumes bool) error
	}

	Event struct {
		ContainerId string `json:"id"`
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
		Config          struct {
			Image        string
			AttachStderr bool
			AttachStdin  bool
			AttachStdout bool
		}
		HostConfig struct {
			PortBindings map[string][]Binding
		}
	}

	dockerClient struct {
		path string
	}

	DaemonInfo struct {
		Containers         int
		Debug              int
		Driver             string
		DriverStatus       [][]string
		ExecutionDriver    string
		IPv4Forwarding     int
		Images             int
		IndexServerAddress string
		InitPath           string
		INitSha1           string
		KernelVersion      string
		MemoryLimit        int
		NEventsListener    int
		NFd                int
		NGoroutines        int
		Sockets            []string
		SwapLimit          int
	}
)

func (d *DaemonInfo) RootPath() string {
	for _, i := range d.DriverStatus {
		if i[0] == "Root Dir" {
			return i[1]
		}
	}
	return ""
}

func NewClient(path string) (Docker, error) {
	return &dockerClient{path}, nil
}

func (d *dockerClient) newConn() (*httputil.ClientConn, error) {
	proto, path := utils.ParseURL(d.path)
	conn, err := net.Dial(proto, path)

	if err != nil {
		return nil, err
	}
	return httputil.NewClientConn(conn, nil), nil
}

func (docker *dockerClient) PullImage(name string) error {
	var (
		method = "POST"
		uri    = fmt.Sprintf("/images/create?fromImage=%s", name)
	)

	respBody, err := docker.newRequest(method, uri, nil)
	if err != nil {
		return nil
	}
	defer respBody.Close()

	return nil
}

func (docker *dockerClient) RemoveContainer(name string, force, volumes bool) error {
	var (
		method = "DELETE"
		uri    = fmt.Sprintf("/containers/%s?force=%s&volumes=%s", name, force, volumes)
	)

	respBody, err := docker.newRequest(method, uri, nil)
	if err != nil {
		return err
	}
	defer respBody.Close()

	return nil
}

func (docker *dockerClient) CreateContainer(container map[string]interface{}) (string, error) {
	var (
		method = "POST"
		name   string
		uri    = fmt.Sprintf("/containers/create?name=%s", container["Name"])
	)

	delete(container, "Name")
	respBody, err := docker.newRequest(method, uri, container)
	if err != nil {
		return "", err
	}
	defer respBody.Close()

	type createResp struct {
		Id string
	}
	var respData createResp
	err = json.NewDecoder(respBody).Decode(&respData)
	if err != nil {
		return name, err
	}
	name = respData.Id

	return name, nil
}

func (docker *dockerClient) StartContainer(name string, hostConfig interface{}) error {
	var (
		method = "POST"
		uri    = fmt.Sprintf("/containers/%s/start", name)
	)

	respBody, err := docker.newRequest(method, uri, hostConfig)
	if err != nil {
		return err
	}
	defer respBody.Close()

	return nil
}

func (docker *dockerClient) RunContainer(config map[string]interface{}) (string, error) {

	name, err := docker.CreateContainer(config)
	if err != nil {
		return "", err
	}

	return name, docker.StartContainer(name, config["HostConfig"])
}

func (docker *dockerClient) FetchContainer(name string) (*Container, error) {
	var (
		method = "GET"
		uri    = fmt.Sprintf("/containers/%s/json", name)
	)

	respBody, err := docker.newRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()
	var container *Container
	err = json.NewDecoder(respBody).Decode(&container)
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

	respBody, err := docker.newRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var containers []*Container
	if err = json.NewDecoder(respBody).Decode(&containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func (docker *dockerClient) newRequest(method, uri string, body interface{}) (io.ReadCloser, error) {
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, uri, bytes.NewBuffer(bodyJson))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	c, err := docker.newConn()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if docker.isOkStatus(resp.StatusCode) {
		return resp.Body, nil
	}
	return nil, fmt.Errorf("invalid HTTP request %d %s", resp.StatusCode, resp.Status)
}

func (d *dockerClient) isOkStatus(code int) bool {
	codes := map[int]bool{
		200: true,
		201: true,
		204: true,
		400: false,
		404: false,
		500: false,
		409: false,
		406: false,
	}

	return codes[code]
}

func (docker *dockerClient) Info() (*DaemonInfo, error) {
	var (
		method = "GET"
		uri    = "/info"
	)

	respBody, err := docker.newRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var info *DaemonInfo
	if err = json.NewDecoder(respBody).Decode(&info); err != nil {
		return nil, err
	}
	return info, nil
}

func (d *dockerClient) GetEvents() chan *Event {
	eventChan := make(chan *Event, 100) // 100 event buffer
	go func() {
		defer close(eventChan)

		c, err := d.newConn()
		if err != nil {
			log.Printf("cannot connect to docker: %s", err)
			return
		}
		defer c.Close()

		req, err := http.NewRequest("GET", "/events", nil)
		if err != nil {
			log.Printf("bad request for events: %s", err)
			return
		}

		resp, err := c.Do(req)
		if err != nil {
			log.Printf("cannot connect to events endpoint: %s", err)
			return
		}
		defer resp.Body.Close()

		// handle signals to stop the socket
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		go func() {
			for sig := range sigChan {
				log.Printf("received signal '%v', exiting", sig)

				c.Close()
				close(eventChan)
				os.Exit(0)
			}
		}()

		dec := json.NewDecoder(resp.Body)
		for {
			var event *Event
			if err := dec.Decode(&event); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("cannot decode json: %s", err)
				continue
			}
			eventChan <- event
		}
		log.Printf("closing event channel")
	}()
	return eventChan
}

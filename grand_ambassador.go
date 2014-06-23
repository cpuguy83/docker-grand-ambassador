package main

import (
	"flag"
	"fmt"
	"github.com/cpuguy83/docker-grand-ambassador/docker"
	"github.com/cpuguy83/docker-grand-ambassador/gocat"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
	"log"
	"os"
)

var (
	dockerClient docker.Docker
)

func main() {
	var (
		socket        = flag.String("sock", "/var/run/docker.sock", "Path to docker socket")
		containerName = flag.String("name", "", "Name/ID of container to ambassadorize")
	)
	flag.Parse()
	dockerClient, err := docker.NewClient(*socket)
	if err != nil {
		log.Printf("Could not connect to Docker: %s", err)
		os.Exit(1)
	}
	container, err := dockerClient.FetchContainer(*containerName)
	if err != nil {
		log.Printf("%v", err)
		os.Exit(2)
	}
	quit := make(chan bool)
	log.Printf("Initializing proxy")
	go proxyContainer(container, quit)

	events := dockerClient.GetEvents()
	go handleEvents(container, events, quit)

	wait := make(chan bool)
	<-wait
}

func handleEvents(container *docker.Container, eventChan chan *docker.Event, quit chan bool) error {
	log.Printf("Handling Events for: %v: %v", container.Id, container.Name)
	for event := range eventChan {
		if container.Id == event.ContainerId {
			log.Printf("Received event: %v", event)
			switch event.Status {
			case "die", "stop", "kill":
				log.Printf("Handling event for stop/die/kill")
				quit <- true
			case "start", "restart":
				log.Printf("Handling event start/restart")
				c, err := dockerClient.FetchContainer(event.ContainerId)
				if err != nil {
					return err
				}
				quit <- true
				go proxyContainer(c, quit)
			default:
				log.Printf("Not handling event: %v", event)
			}
		}
	}
	log.Printf("Stopped handling events")
	return nil
}

func proxyContainer(container *docker.Container, quit chan bool) {
	ip := container.NetworkSettings.IpAddress
	ports := container.NetworkSettings.Ports
	if len(ports) != 0 {
		for key, _ := range ports {
			port, proto := utils.SplitPort(key)
			go proxy(ip, port, proto, quit)
		}
	}
}

func proxy(ip, port, proto string, quit chan bool) {
	local := fmt.Sprintf("%v://0.0.0.0:%v", proto, port)
	remote := fmt.Sprintf("%v://%v:%v", proto, ip, port)
	out := fmt.Sprintf("Proxying %s:%s/%s", ip, port, proto)
	log.Printf(out)
	gocat.NewProxy(local, remote, quit)
}

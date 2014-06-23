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
	go proxyContainer(container, quit)

	events := dockerClient.GetEvents()
	go handleEvents(container, events, quit)

	wait := make(chan bool)
	<-wait
}

func handleEvents(container *docker.Container, eventChan chan *docker.Event, quit chan bool) error {
	log.Printf("Handling Events for: %v", container.Id)
	for event := range eventChan {
		if container.Id == event.ContainerID {
			log.Printf("Received event: %v", event)
			switch event.Status {
			case "die", "stop", "kill":
				quit <- true
				log.Printf("Handling event %v", event)
			case "start", "restart":
				log.Printf("Handling event %v", event)
				quit <- true
				container, err := dockerClient.FetchContainer(event.ContainerID)
				if err != nil {
					return err
				}
				go proxyContainer(container, quit)
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
			out := fmt.Sprintf("Proxying %s:%s/%s", ip, port, proto)
			go proxy(ip, port, proto, quit)
			log.Printf(out)
		}
	}
}

func proxy(ip, port, proto string, quit chan bool) {
	local := fmt.Sprintf("%v://0.0.0.0:%v", proto, port)
	remote := fmt.Sprintf("%v://%v:%v", proto, ip, port)
	gocat.NewProxy(local, remote, quit)
}

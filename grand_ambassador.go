package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/cpuguy83/docker-grand-ambassador/docker"
	"github.com/cpuguy83/docker-grand-ambassador/proxy"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
)

var (
	dockerClient docker.Docker
)

func main() {
	var (
		socket        = flag.String("sock", "/var/run/docker.sock", "Path to docker socket")
		containerName = flag.String("name", "", "Name/ID of container to ambassadorize")
		err           error
	)

	flag.Parse()

	if *containerName == "" {
		fmt.Println("Missing required arguments")
		os.Exit(1)
	}

	dockerClient, err := docker.NewClient(*socket)
	if err != nil {
		log.Printf("Could not connect to Docker: %s", err)
		os.Exit(1)
	}
	container, err := dockerClient.FetchContainer(*containerName)
	if err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}

	proxyChan := makeProxyChan(container)

	log.Printf("Initializing proxy")
	if err = proxyContainer(container, proxyChan); err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}

	events := dockerClient.GetEvents()
	go handleEvents(container, events, dockerClient, proxyChan)

	wait := make(chan bool)
	<-wait
}

func handleEvents(container *docker.Container, eventChan chan *docker.Event, dockerClient docker.Docker, proxyChan chan net.Listener) {
	log.Printf("Handling Events for: %v: %v", container.Id, container.Name)
	for event := range eventChan {
		if container.Id == event.ContainerId {
			log.Printf("Received event: %v", event)
			switch event.Status {
			case "die", "stop", "kill":
				log.Printf("Handling event for stop/die/kill")
				for srv := range proxyChan {
					srv.Close()
				}
			case "start", "restart":
				log.Printf("Handling event start/restart")
				c, err := dockerClient.FetchContainer(event.ContainerId)
				if err != nil {
					log.Printf("%v", err)
					os.Exit(2)
				}
				log.Printf("Closing old servers")
				for srv := range proxyChan {
					srv.Close()
				}
				log.Printf("Servers closed")
				proxyChan = makeProxyChan(container)
				if err = proxyContainer(c, proxyChan); err != nil {
					log.Printf("%v", err)
					os.Exit(2)
				}
			default:
				log.Printf("Not handling event: %v", event)
			}
		}
	}
	log.Printf("Stopped handling events")
}

func proxyContainer(container *docker.Container, proxyChan chan net.Listener) error {
	ip := container.NetworkSettings.IpAddress
	ports := container.NetworkSettings.Ports
	if len(ports) != 0 {
		for key, _ := range ports {
			port, proto := utils.SplitPort(key)
			local := fmt.Sprintf("%v://0.0.0.0:%v", proto, port)
			remote := fmt.Sprintf("%v://%v:%v", proto, ip, port)
			out := fmt.Sprintf("Proxying %s:%s/%s", ip, port, proto)
			log.Printf(out)
			srv, err := proxy.NewProxy(local, remote)
			if err != nil {
				return err
			}
			proxyChan <- srv
		}
	}
	close(proxyChan)
	return nil
}

func makeProxyChan(container *docker.Container) chan net.Listener {
	return make(chan net.Listener, len(container.NetworkSettings.Ports))
}

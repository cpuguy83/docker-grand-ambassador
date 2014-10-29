package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
	"github.com/cpuguy83/dockerclient"
	"github.com/docker/docker/pkg/proxy"
)

var (
	dockerClient docker.Docker
)

func main() {
	var (
		socket        = flag.String("sock", "/var/run/docker.sock", "Path to docker socket")
		containerName = flag.String("name", "", "Name/ID of container to ambassadorize")
		containerWait = flag.Bool("wait", true, "Wait for container to be created if it doesn't exist on start")
		err           error
	)

	flag.Parse()

	if *containerName == "" {
		log.Fatalf("Missing required arguments")
	}

	if !strings.HasPrefix(*socket, "tcp") {
		if _, err := os.Stat(*socket); err != nil {
			fmt.Fprintln(os.Stderr,
				`
You must mount your docker socket into the container. Use the following command:

	docker run -v /var/run/docker.sock:/var/run/docker.sock -d cpuguy83/docker-grand-ambassador -name `+*containerName+`

`)
			return
		}
	}

	dockerClient, err := docker.NewClient(*socket)
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}
	events := dockerClient.GetEvents()
	container, err := dockerClient.FetchContainer(*containerName)
	if err != nil {
		log.Println("Container does not exist", *containerName)
		if !*containerWait {
			log.Fatal("Not waiting for container, exiting")
		}

		container = waitForContainer(*containerName, events, dockerClient)
	}

	proxyChan := makeProxyChan(container)

	log.Printf("Initializing proxy")
	if err = proxyContainer(container, proxyChan); err != nil {
		log.Fatal(err)
	}

	go handleEvents(container, events, dockerClient, proxyChan)

	wait := make(chan struct{})
	<-wait
}

func waitForContainer(name string, eventChan chan *docker.Event, client docker.Docker) *docker.Container {
	log.Println("Waiting for container to be created:", name)
	var (
		c   *docker.Container
		err error
	)
	for event := range eventChan {
		c, err = client.FetchContainer(event.ContainerId)
		if err != nil {
			continue
		}

		if strings.TrimPrefix(c.Name, "/") == name {
			break
		}
	}

	for !c.State.Running {
		c, err = client.FetchContainer(c.Id)
		if err != nil {
			log.Infof("Error waiting for contianer start:", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	return c
}

func handleEvents(container *docker.Container, eventChan chan *docker.Event, dockerClient docker.Docker, proxyChan chan proxy.Proxy) {
	log.Printf("Handling Events for: %v: %v", container.Id, container.Name)
	for event := range eventChan {
		c, err := dockerClient.FetchContainer(event.ContainerId)
		if err != nil {
			if event.ContainerId != container.Id {
				continue
			}

			c = container
		}
		if container.Name == c.Name {
			// Set the container to match
			// This is so we can recover properly if a our container was removed
			container = c

			log.Printf("Received event: %v", event)
			switch event.Status {
			case "die", "stop", "kill":
				log.Debugf("Handling event for stop/die/kill")
				for srv := range proxyChan {
					srv.Close()
				}
			case "start", "restart":
				log.Debugf("Handling event start/restart")
				log.Printf("Closing old servers")
				for srv := range proxyChan {
					srv.Close()
				}
				time.Sleep(300 * time.Millisecond)
				log.Printf("Servers closed")
				proxyChan = makeProxyChan(container)
				if err = proxyContainer(c, proxyChan); err != nil {
					log.Fatal(err)
				}
			default:
				log.Debugf("Not handling event: %q", event)
			}
		}
	}
	log.Printf("Stopped handling events")
}

func proxyContainer(container *docker.Container, proxyChan chan proxy.Proxy) error {
	defer close(proxyChan)
	ip := container.NetworkSettings.IpAddress
	ports := container.NetworkSettings.Ports
	if len(ports) == 0 {
		log.Infof("No ports to proxy")
		return nil
	}
	for key, _ := range ports {
		port, proto := utils.SplitPort(key)
		var (
			rAddr net.Addr
			lAddr net.Addr
			err   error
		)
		switch proto {
		case "tcp", "tcp6", "tcp4":
			rAddr, err = net.ResolveTCPAddr(proto, fmt.Sprintf("%s:%s", ip, port))
			if err != nil {
				return err
			}
			lAddr, err = net.ResolveTCPAddr(proto, fmt.Sprintf("0.0.0.0:%s", port))
			if err != nil {
				return err
			}
		case "udp", "udp4", "udp6":
			rAddr, err = net.ResolveUDPAddr(proto, fmt.Sprintf("%s:%s", ip, port))
			if err != nil {
				return err
			}
			lAddr, err = net.ResolveUDPAddr(proto, fmt.Sprintf("0.0.0.0:%s", port))
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unsupported proto: %s", proto)
		}

		out := fmt.Sprintf("Proxying %s:%s/%s", ip, port, proto)
		log.Printf(out)
		srv, err := proxy.NewProxy(lAddr, rAddr)
		if err != nil {
			return err
		}
		go srv.Run()
		proxyChan <- srv
	}
	return nil
}

func makeProxyChan(container *docker.Container) chan proxy.Proxy {
	return make(chan proxy.Proxy, len(container.NetworkSettings.Ports))
}

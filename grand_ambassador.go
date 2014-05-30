package main

import (
	"./docker"
	"./utils"
	"flag"
	"fmt"
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
		fmt.Println("Could not connect to Docker: %s", err)
		os.Exit(1)
	}
	container, err := dockerClient.FetchContainer(*containerName)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	ip := container.NetworkSettings.IpAddress
	ports := container.NetworkSettings.Ports
	if len(ports) != 0 {
		for key, _ := range ports {
			port, proto := utils.SplitPort(key)
			out := fmt.Sprintf("Proxying %s:%s/%s", ip, port, proto)
			proxy(ip, port, proto)
			fmt.Println(out)
		}
	}
}

func proxy(ip, port, proto string) {

}

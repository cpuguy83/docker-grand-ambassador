package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
	"github.com/cpuguy83/dockerclient"
	"github.com/docker/docker/pkg/proxy"
)

var (
	dockerClient docker.Docker
)

type Names []string

func (n *Names) Set(value string) error {
	*n = append(*n, value)
	return nil
}
func (n *Names) String() string {
	return fmt.Sprintf("%d", *n)
}

type containerProxy struct {
	s    []proxy.Proxy
	name string
	id   string
}

func (p *containerProxy) Close() {
	log.Infof("Stopping proxy for %s", p.name)
	for _, pp := range p.s {
		pp.Close()
	}
}

func (p *containerProxy) Serve() {
	log.Infof("Starting proxy for %s", p.name)
	for _, pp := range p.s {
		go pp.Run()
	}
}

type proxyRepo struct {
	l       sync.Mutex
	nameIdx map[string]*containerProxy
	idIdx   map[string]*containerProxy
}

func (r *proxyRepo) find(name string) *containerProxy {
	if p, exists := r.nameIdx[name]; exists {
		return p
	}
	return r.idIdx[name]
}

func (r *proxyRepo) Add(p *containerProxy) {
	r.l.Lock()
	defer r.l.Unlock()
	if pp := r.find(p.name); pp != nil {
		log.Infof("Proxy for %s already exists, replacing", p.name)
		r.stop(p.name)
	}
	r.nameIdx[p.name] = p
	r.idIdx[p.id] = p
}

func (r *proxyRepo) stop(name string) {
	if p := r.find(name); p != nil {
		p.Close()
		r.remove(p.name)
	}
}

func (r *proxyRepo) remove(name string) {
	if p := r.find(name); p != nil {
		delete(r.idIdx, p.id)
		delete(r.nameIdx, p.name)
	}
}

func (r *proxyRepo) Stop(name string) {
	r.l.Lock()
	defer r.l.Unlock()
	r.stop(name)
}

func newProxyRepo() *proxyRepo {
	return &proxyRepo{
		nameIdx: make(map[string]*containerProxy),
		idIdx:   make(map[string]*containerProxy),
	}
}

func main() {
	var certPath = os.Getenv("DOCKER_CERT_PATH")
	if certPath == "" {
		certPath = filepath.Join(os.Getenv("HOME"), ".docker")
	}
	var (
		n             Names
		socket        = flag.String("sock", "/var/run/docker.sock", "Path to docker socket")
		containerWait = flag.Bool("wait", true, "Wait for container to be created if it doesn't exist on start")
		flTls         = flag.Bool("tls", false, "Enable TLS for connecting to Docker socket")
		flTlsVerify   = flag.Bool("tlsverify", false, "Enable TLS verification of the Docker host")
		flTlsCert     = flag.String("tlscert", filepath.Join(certPath, "cert.pem"), "Path to TLS cert")
		flTlsKey      = flag.String("tlskey", filepath.Join(certPath, "key.pem"), "Path to TLS key")
		flTlsCa       = flag.String("tlscacert", filepath.Join(certPath, "ca.pem"), "Path to TLS ca cert")
		flLogLevel    = flag.String("log-level", "info", "Set debug logging")
		err           error
	)

	if !*containerWait {
		log.Warnf("-wait flag is deprecated and will not be used")
	}
	flag.Var(&n, "name", "Name/ID of container to ambassadorize")

	flag.Parse()
	if len(n) == 0 {
		log.Fatalf("Missing required arguments *name*")
	}

	var logLevel log.Level
	logLevel, err = log.ParseLevel(*flLogLevel)
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	var names = make(map[string]struct{})
	for _, name := range n {
		names[name] = struct{}{}
	}

	if err := checkSocket(*socket); err != nil {
		fmt.Fprintln(os.Stderr,
			`
You must mount your docker socket into the container. Use the following command:

	docker run -v /var/run/docker.sock:/var/run/docker.sock -d cpuguy83/docker-grand-ambassador -name `+n[0]+`

`)
	}

	dockerClient, err := docker.NewClient(*socket)
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}
	if strings.HasPrefix(*socket, "tcp") && (*flTls || *flTlsVerify) {
		tlsConfig, err := getTlsConfig(*flTlsVerify, *flTlsCert, *flTlsKey, *flTlsCa)
		if err != nil {
			log.Fatalf("Error setting up TLS: %v", err)
		}
		dockerClient.SetTlsConfig(tlsConfig)
	}

	proxies := newProxyRepo()
	for name := range names {
		p, err := newContainerProxy(name, dockerClient)
		if err != nil {
			log.Info(err)
			continue
		}
		proxies.Add(p)
		go p.Serve()
	}

	handleEvents(names, proxies, dockerClient)
}

func handleEvents(names map[string]struct{}, proxies *proxyRepo, client docker.Docker) {
	events := client.GetEvents()
	for event := range events {
		log.Debugf("Received event: %v", event)
		switch event.Status {
		case "die", "stop", "kill":
			proxies.Stop(event.ContainerId)
		case "start", "restart":
			c, _ := client.FetchContainer(event.ContainerId)
			if c == nil {
				continue
			}
			name := strings.TrimPrefix(c.Name, "/")

			if _, exists := names[name]; !exists {
				log.Debugf("Ignoring event for %s", name)
				continue
			}
			log.Debugf("Handling event %s start/restart", name)
			proxies.Stop(name)
			time.Sleep(300 * time.Millisecond)
			if err := waitForContainerStart(name, client); err != nil {
				log.Error(err)
			}

			p, err := newContainerProxy(name, client)
			if err != nil {
				log.Info(err)
				continue
			}
			proxies.Add(p)
			go p.Serve()
		default:
			log.Debugf("Not handling event: %q", event)
		}
	}
}

func waitForContainerStart(name string, client docker.Docker) error {
	for {
		log.Debugf("Waiting for container %s to start", name)
		c, err := client.FetchContainer(name)
		if err != nil {
			return fmt.Errorf("Error waiting for container, %v", err)
		}
		if c.State.Running {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func newContainerProxy(name string, client docker.Docker) (*containerProxy, error) {
	container, err := client.FetchContainer(name)
	if container == nil || err != nil || !container.State.Running {
		return nil, fmt.Errorf("Container %s does not exist or is not running, skipping for now", name)
	}
	ip := container.NetworkSettings.IpAddress
	ports := container.NetworkSettings.Ports
	if len(ports) == 0 {
		return nil, fmt.Errorf("No ports to proxy for %s", name)
	}
	var proxies []proxy.Proxy
	for key, _ := range ports {
		port, proto := utils.SplitPort(key)
		p, err := newProxy(proto, ip, port)
		if err != nil {
			log.Info("Error creating proxy for %s %s:%s - %v", name, ip, port, err)
			continue
		}
		proxies = append(proxies, p)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("Error creating proxies for container %s", name)
	}
	return &containerProxy{id: container.Id, name: name, s: proxies}, nil
}

func newProxy(proto, ip, port string) (proxy.Proxy, error) {
	var (
		rAddr net.Addr
		lAddr net.Addr
		err   error
	)
	switch proto {
	case "tcp", "tcp6", "tcp4":
		rAddr, err = net.ResolveTCPAddr(proto, fmt.Sprintf("%s:%s", ip, port))
		if err != nil {
			return nil, err
		}
		lAddr, err = net.ResolveTCPAddr(proto, fmt.Sprintf("0.0.0.0:%s", port))
		if err != nil {
			return nil, err
		}
	case "udp", "udp4", "udp6":
		rAddr, err = net.ResolveUDPAddr(proto, fmt.Sprintf("%s:%s", ip, port))
		if err != nil {
			return nil, err
		}
		lAddr, err = net.ResolveUDPAddr(proto, fmt.Sprintf("0.0.0.0:%s", port))
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unsupported proto: %s", proto)
	}
	return proxy.NewProxy(lAddr, rAddr)
}

func makeProxyChan(container *docker.Container) chan proxy.Proxy {
	return make(chan proxy.Proxy, len(container.NetworkSettings.Ports))
}

func getTlsConfig(verify bool, cert, key, ca string) (*tls.Config, error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	if verify {
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(file)
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}

	_, errCert := os.Stat(cert)
	_, errKey := os.Stat(key)
	if errCert == nil || errKey == nil {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("Couldn't load X509 key pair: %v. Key encrpyted?\n", err)
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}
	config.MinVersion = tls.VersionTLS10

	return &config, nil
}

func checkSocket(socket string) error {
	if !strings.HasPrefix(socket, "tcp") {
		if _, err := os.Stat(socket); err != nil {
			return err
		}
	}
	return nil
}

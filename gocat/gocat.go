package gocat

import (
	"fmt"
	"github.com/cpuguy83/docker-grand-ambassador/utils"
	"net"
	"os"
	"strings"
)

type host struct {
	Proto   string
	Address string
	Port    int
}

func NewProxy(fromUrl, toUrl string) error {
	var (
		from host
		to   host
	)

	from.Proto, from.Address = parseURL(fromUrl)
	to.Proto, to.Address = utils.ParseURL(toUrl)

	waiting, complete := make(chan net.Conn), make(chan net.Conn)

	server, err := net.Listen(from.Proto, from.Address)
	if err != nil {
		return err
	}

	for {
		conn, err := server.Accept()
		if err != nil {
			return err
		}
		go func() {
			go handleConn(waiting, complete, to)
			waiting <- conn
		}()
	}

	return nil
}

func closeConn(in chan net.Conn) {
	for conn := range in {
		conn.Close()
	}
}

func handleConn(waiting chan net.Conn, complete chan net.Conn, remote host) {
	for conn := range waiting {
		proxyConn(remote, conn)
		complete <- conn
	}
}

func proxyConn(toHost host, from net.Conn) {
	defer from.Close()

	to, err := net.Dial(toHost.Proto, toHost.Address)
	if err != nil {
		fmt.Errorf("%v", err)
		return
	}
	defer to.Close()

	complete := make(chan bool)

	go copyContent(from, to, complete)
	go copyContent(to, from, complete)
	<-complete
}

func copyContent(from, to net.Conn, complete chan bool) {
	var (
		err   error  = nil
		bytes []byte = make([]byte, 256)
		read  int    = 0
	)

	for {
		read, err = from.Read(bytes)
		if err != nil {
			complete <- true
			break
		}
		_, err = to.Write(bytes[:read])
		if err != nil {
			complete <- true
			break
		}
	}
}

func main() {
	ints, _ := net.Interfaces()
	fmt.Printf("%s", ints)
	from := os.Args[1]
	to := os.Args[2]

	err := NewProxy(from, to)
	if err != nil {
		fmt.Println(err)
	}
}

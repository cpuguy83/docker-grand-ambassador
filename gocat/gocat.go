package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

type proxyHost struct {
	Proto   string
	Address string
}

func NewProxy(localAddr, remoteAddr string) error {
	var (
		local  proxyHost
		remote proxyHost
	)
	local.Proto, local.Address = splitURI(localAddr)
	remote.Proto, remote.Address = splitURI(remoteAddr)
	fmt.Println(local, remote)

	listener, err := net.Listen(local.Proto, local.Address)
	if err != nil {
		return err
	}

	fmt.Println(listener)

	pending, complete := make(chan net.Conn), make(chan net.Conn)
	go closeConn(complete)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func() {
			go handleConn(pending, complete, remote)
			pending <- conn
		}()
	}

	return nil
}

func closeConn(in <-chan net.Conn) {
	for conn := range in {
		conn.Close()
	}
}

func handleConn(in <-chan net.Conn, out chan<- net.Conn, remote proxyHost) {
	for conn := range in {
		proxyConn(remote, conn)
		out <- conn
	}
}

func proxyConn(host proxyHost, conn net.Conn) {
	rConn, err := net.Dial(host.Proto, host.Address)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	buf := &bytes.Buffer{}
	for {
		data := make([]byte, 256)
		n, err := conn.Read(data)
		if err != nil {
			if err != io.EOF {
				panic(err)
			}
		}
		buf.Write(data[:n])
		if (data[len(data[:n])-2] == '\r' && data[len(data[:n])-1] == '\n') || data[0] == 0 {
			break
		}
	}

	if _, err := rConn.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	data := make([]byte, 1024)
	n, err := rConn.Read(data)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
	}
	fmt.Println(n)
}

func splitURI(uri string) (string, string) {
	arr := strings.Split(uri, "://")

	if len(arr) == 1 {
		return "unix", arr[0]
	}

	proto := arr[0]
	if proto == "http" {
		proto = "tcp"
	}

	return proto, arr[1]
}

func main() {
	local := "tcp://localhost:3000"
	remote := "tcp://google.com:80"

	err := NewProxy(local, remote)
	if err != nil {
		fmt.Println(err)
	}
}

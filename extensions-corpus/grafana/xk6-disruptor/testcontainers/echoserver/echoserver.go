// Package main contains a simple TCP echoserver, that repeats back to the client every line it receives.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

func main() {
	lAddr := &net.TCPAddr{
		Port: 6666,
	}

	srv, err := net.ListenTCP("tcp4", lAddr)
	if err != nil {
		log.Fatalf("setting up listener: %v", err)
	}

	log.Printf("Listening on %s", lAddr)

	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Fatalf("accepting connection: %v", err)
		}

		go func() {
			log.Printf("received connection from %s", conn.RemoteAddr())
			err := echo(conn)
			if errors.Is(err, io.EOF) {
				log.Printf("%s closed the connection", conn.RemoteAddr())
			} else if err != nil {
				log.Printf("%s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

func echo(conn net.Conn) error {
	lineReader := bufio.NewReader(conn)
	for {
		line, err := lineReader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading from peer: %w", err)
		}

		log.Printf("%s: %s", conn.RemoteAddr(), strings.TrimSpace(line))
		_, err = conn.Write([]byte(line))
		if err != nil {
			return fmt.Errorf("echoing back to peer: %w", err)
		}
	}
}

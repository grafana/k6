//go:build codegen

// Package main contains codegen tool.
package main

import (
	"bytes"
	"log"
	"os"
)

func usage() {
	log.Fatal("error: usage: codegen {json|ts|test} filename")
}

//nolint:forbidigo
func main() {
	if len(os.Args) != 3 {
		usage()
	}

	command := os.Args[1]

	var (
		buff bytes.Buffer
		err  error
		file *os.File
	)

	switch command {
	case "it":
		err = itGen(&buff)
	case "test":
		err = goTest(&buff)
	case "ts":
		err = tsGen(&buff)
	case "json":
		err = jsonGen(&buff)
	default:
		usage()
	}

	if err != nil {
		log.Fatalf("error: %s", err.Error())
	}

	file, err = os.Create(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}

	_, err = file.Write(buff.Bytes())
	if err != nil {
		log.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

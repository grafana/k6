// Package main is a simple command runner that executes a specified command
// and manages an embedded MQTT broker.
// It sets up the broker and sets environment variable MQTT_BROKER_ADDRESS,
// and then starts the command.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/grafana/xk6-mqtt/internal/broker"
)

func main() {
	//nolint:forbidigo,gosec // CLI helper tool: argv/stderr access and the usage message are intended
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		os.Exit(1)
	}

	// Set up the embedded MQTT broker
	server := broker.Setup()

	// Extract command and arguments
	cmdName := os.Args[1]  //nolint:forbidigo // standalone CLI helper tool; os access is intended
	cmdArgs := os.Args[2:] //nolint:forbidigo // standalone CLI helper tool; os access is intended

	// Create the command
	cmd := exec.CommandContext(context.TODO(), cmdName, cmdArgs...) //#nosec G204,G702

	// Connect stdin, stdout, stderr to preserve interactivity
	cmd.Stdin = os.Stdin   //nolint:forbidigo // standalone CLI helper tool; os access is intended
	cmd.Stdout = os.Stdout //nolint:forbidigo // standalone CLI helper tool; os access is intended
	cmd.Stderr = os.Stderr //nolint:forbidigo // standalone CLI helper tool; os access is intended

	// Handle signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the command
	//nolint:forbidigo,gosec // CLI helper tool: stderr access and the error message are intended
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start command '%s': %v\n", cmdName, err)
		os.Exit(1)
	}

	// Handle signal forwarding to child process
	go func() {
		sig := <-sigChan

		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
	}()

	// Wait for command to complete
	err := cmd.Wait()
	//nolint:forbidigo // CLI helper tool: os.Exit/stderr access is intended
	if err != nil {
		var exitError *exec.ExitError

		if errors.As(err, &exitError) {
			// Command exited with non-zero status
			os.Exit(exitError.ExitCode())
		}

		fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		os.Exit(1)
	}

	log.Print("Shutting down embedded MQTT broker...")

	if err := server.Close(); err != nil {
		log.Fatalf("Error shutting down embedded broker: %v", err)
	}
}

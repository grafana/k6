// Package main provides a command runner that executes commands with embedded TCP and HTTP echo servers.
// The program starts both a TCP echo server and an HTTP echo server on random localhost ports,
// sets the TCP_ECHO_HOST, TCP_ECHO_PORT, HTTP_ECHO_HOST, HTTP_ECHO_PORT, and HTTP_ECHO_URL
// environment variables, then executes the specified command with arguments.
// The echo servers are automatically cleaned up when the command completes.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/grafana/xk6-tcp/internal/echo"
)

// CommandRunner handles command execution with echo server integration.
type CommandRunner struct {
	server *echo.Server
}

func main() {
	os.Exit(run()) //nolint:forbidigo // standalone CLI helper tool; os access is intended
}

func run() int {
	if len(os.Args) == 1 { //nolint:forbidigo // standalone CLI helper tool; os access is intended
		printUsage()

		return 1
	}

	runner, err := NewCommandRunner()
	if err != nil {
		slog.Error("Failed to initialize command runner", "err", err)

		return 1
	}
	defer runner.Shutdown()

	return runner.Run(os.Args[1], os.Args[2:]...) //nolint:forbidigo // standalone CLI helper tool; os access is intended
}

// printUsage prints the command usage information.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0]) //nolint:forbidigo,gosec // CLI usage, not XSS
}

// NewCommandRunner creates a new CommandRunner with embedded TCP and HTTP echo servers.
func NewCommandRunner() (*CommandRunner, error) {
	server, err := echo.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create echo servers: %w", err)
	}

	server.Start()

	// Set all environment variables for child processes
	if err := server.Setenv(); err != nil {
		return nil, err
	}

	return &CommandRunner{server: server}, nil
}

// Run executes the specified command with arguments.
func (cr *CommandRunner) Run(cmdName string, cmdArgs ...string) int {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...) //#nosec G702,G204

	// Connect stdin, stdout, stderr to preserve interactivity
	cmd.Stdin = os.Stdin   //nolint:forbidigo // standalone CLI helper tool; os access is intended
	cmd.Stdout = os.Stdout //nolint:forbidigo // standalone CLI helper tool; os access is intended
	cmd.Stderr = os.Stderr //nolint:forbidigo // standalone CLI helper tool; os access is intended

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the command
	if err := cmd.Start(); err != nil {
		slog.Error("Failed to start command", "cmd", cmdName, "err", err) //#nosec G706

		return 1
	}

	// Create a done channel to signal the signal handler to stop
	done := make(chan struct{})

	// Handle signal forwarding in a goroutine
	go cr.handleSignals(cmd, sigChan, done)

	// Wait for command completion and return appropriate exit code
	exitCode := cr.waitForCommand(cmd)

	// Stop listening for signals and signal the handler goroutine to exit
	signal.Stop(sigChan)
	close(done)

	return exitCode
}

// Shutdown gracefully shuts down the command runner.
func (cr *CommandRunner) Shutdown() {
	if cr.server != nil {
		slog.Info("Shutting down embedded echo servers...")
		cr.server.Stop()
	}
}

// handleSignals forwards signals to the child process.
func (cr *CommandRunner) handleSignals(cmd *exec.Cmd, sigChan <-chan os.Signal, done <-chan struct{}) {
	for {
		select {
		case sig := <-sigChan:
			if cmd.Process != nil {
				if err := cmd.Process.Signal(sig); err != nil {
					slog.Debug("Failed to forward signal to child process", "signal", sig, "err", err)
				}
			}
		case <-done:
			return
		}
	}
}

// waitForCommand waits for command completion and returns exit code.
func (cr *CommandRunner) waitForCommand(cmd *exec.Cmd) int {
	err := cmd.Wait()
	if err == nil {
		return 0
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}

	slog.Error("Command execution error", "err", err)

	return 1
}

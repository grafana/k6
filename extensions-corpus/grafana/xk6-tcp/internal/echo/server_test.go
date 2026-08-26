package echo

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestEchoServer(t *testing.T) {
	t.Parallel()

	// Create a new echo server
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create echo server: %v", err)
	}

	// Start the server
	server.Start()
	defer server.Stop()

	// Get the server address
	addr := server.Address()
	if addr == "" {
		t.Fatal("Server address is empty")
	}

	// Connect to the server
	dialer := &net.Dialer{}

	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to echo server: %v", err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Error closing connection: %v", err)
		}
	}()

	// Test data to send
	testData := "Hello, Echo Server!"

	// Send data to the server
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Failed to write to connection: %v", err)
	}

	// Read the echoed data back
	buffer := make([]byte, len(testData))

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Logf("Warning: Failed to set read deadline: %v", err)
	}

	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from connection: %v", err)
	}

	// Verify the echoed data
	receivedData := string(buffer[:n])
	if receivedData != testData {
		t.Errorf("Expected %q, got %q", testData, receivedData)
	}
}

package netext

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnCloseWrite(t *testing.T) {
	t.Parallel()

	t.Run("tcp", func(t *testing.T) {
		t.Parallel()

		ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })
		go func() {
			peer, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = peer.Close() }()
			got, _ := io.ReadAll(peer) // returns once the client half-closes
			_, _ = peer.Write(append([]byte("echo:"), got...))
		}()

		raw, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", ln.Addr().String())
		require.NoError(t, err)
		var read, written int64
		conn := &Conn{Conn: raw, BytesRead: &read, BytesWritten: &written}
		t.Cleanup(func() { _ = conn.Close() })

		_, err = conn.Write([]byte("ping"))
		require.NoError(t, err)
		require.NoError(t, conn.CloseWrite())
		got, err := io.ReadAll(conn)
		require.NoError(t, err)
		assert.Equal(t, "echo:ping", string(got))
	})

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()

		client, server := net.Pipe()
		t.Cleanup(func() { _ = server.Close() })
		t.Cleanup(func() { _ = client.Close() })
		assert.ErrorIs(t, (&Conn{Conn: client}).CloseWrite(), errors.ErrUnsupported)
	})
}

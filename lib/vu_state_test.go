package lib

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockDialerWithResolver is a mock that implements both DialContexter and AddrResolver
type mockDialerWithResolver struct{}

func (m *mockDialerWithResolver) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	return nil, errors.ErrUnsupported
}

func (m *mockDialerWithResolver) ResolveAddr(_ string) (net.IP, int, error) {
	return nil, 0, errors.ErrUnsupported
}

// mockDialerWithoutResolver is a mock that only implements DialContexter
type mockDialerWithoutResolver struct{}

func (m *mockDialerWithoutResolver) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	return nil, errors.ErrUnsupported
}

func TestGetAddrResolver(t *testing.T) {
	t.Parallel()

	t.Run("returns_same_instance_when_Dialer_implements_AddrResolver", func(t *testing.T) {
		t.Parallel()

		mock := &mockDialerWithResolver{}

		state := &State{
			Dialer: mock,
		}

		resolver := state.GetAddrResolver()
		require.NotNil(t, resolver)
		require.Same(t, mock, resolver)
	})

	t.Run("returns_nil_when_Dialer_does_not_implement_AddrResolver", func(t *testing.T) {
		t.Parallel()

		mock := &mockDialerWithoutResolver{}

		state := &State{
			Dialer: mock,
		}

		resolver := state.GetAddrResolver()
		require.Nil(t, resolver)
	})
}

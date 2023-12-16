//go:build darwin

package quic

import (
	"errors"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/quic-go/quic-go/internal/utils"
)

func setDF(rawConn syscall.RawConn) (bool, error) {
	// Setting DF bit is only supported from macOS11
	// https://github.com/chromium/chromium/blob/117.0.5881.2/net/socket/udp_socket_posix.cc#L555
	if supportsDF, err := isAtLeastMacOS11(); !supportsDF || err != nil {
		return false, err
	}

	// Enabling IP_DONTFRAG will force the kernel to return "sendto: message too long"
	// and the datagram will not be fragmented
	var errDFIPv4, errDFIPv6 error
	if err := rawConn.Control(func(fd uintptr) {
		errDFIPv4 = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_DONTFRAG, 1)
		errDFIPv6 = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_DONTFRAG, 1)
	}); err != nil {
		return false, err
	}
	switch {
	case errDFIPv4 == nil && errDFIPv6 == nil:
		utils.DefaultLogger.Debugf("Setting DF for IPv4 and IPv6.")
	case errDFIPv4 == nil && errDFIPv6 != nil:
		utils.DefaultLogger.Debugf("Setting DF for IPv4.")
	case errDFIPv4 != nil && errDFIPv6 == nil:
		utils.DefaultLogger.Debugf("Setting DF for IPv6.")
		// On macOS, the syscall for setting DF bit for IPv4 fails on dual-stack listeners.
		// Treat the connection as not having DF enabled, even though the DF bit will be set
		// when used for IPv6.
		// See https://github.com/quic-go/quic-go/issues/3793 for details.
		return false, nil
	case errDFIPv4 != nil && errDFIPv6 != nil:
		return false, errors.New("setting DF failed for both IPv4 and IPv6")
	}
	return true, nil
}

func isSendMsgSizeErr(err error) bool {
	return errors.Is(err, unix.EMSGSIZE)
}

func isRecvMsgSizeErr(error) bool { return false }

func isAtLeastMacOS11() (bool, error) {
	uname := &unix.Utsname{}
	err := unix.Uname(uname)
	if err != nil {
		return false, err
	}

	release := string(uname.Release[:])
	if idx := strings.Index(release, "."); idx != -1 {
		version, err := strconv.Atoi(release[:idx])
		if err != nil {
			return false, err
		}
		// Darwin version 20 is macOS version 11
		// https://en.wikipedia.org/wiki/Darwin_(operating_system)#Darwin_20_onwards
		return version >= 20, nil
	}
	return false, nil
}

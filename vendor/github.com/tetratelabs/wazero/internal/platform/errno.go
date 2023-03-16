//go:build !windows

package platform

func adjustErrno(err error) error {
	return err
}

package utils

import (
	"fmt"
	"time"
)

// Retry retries a function until it returns true, error, or the timeout expires.
// If the function returns false, a new attempt is tried after the backoff period
func Retry(timeout time.Duration, backoff time.Duration, f func() (bool, error)) error {
	expired := time.After(timeout)
	for {
		select {
		case <-expired:
			return fmt.Errorf("timeout expired")
		default:
			done, err := f()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			time.Sleep(backoff)
		}
	}
}

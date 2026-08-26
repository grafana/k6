// Package utils offers functions of general utility in other parts of the system
package utils

import (
	"time"
)

// Retry retries a function until it returns true, error, or the timeout expires.
// If the function returns false, a new attempt is tried after the backoff period
func Retry(timeout time.Duration, backoff time.Duration, f func() (bool, error)) (bool, error) {
	expired := time.After(timeout)
	for {
		select {
		case <-expired:
			return false, nil
		default:
			done, err := f()
			if err != nil {
				return false, err
			}
			if done {
				return true, nil
			}
			time.Sleep(backoff)
		}
	}
}

package utils

import (
	"errors"
	"testing"
	"time"
)

func Test_Retry(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title         string
		timeout       time.Duration
		backoff       time.Duration
		failedRetries int
		errorValue    error
		expectError   bool
	}{
		{
			title:         "Succeed on first call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 0,
			errorValue:    nil,
			expectError:   false,
		},
		{
			title:         "Succeed on second call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 1,
			errorValue:    nil,
			expectError:   false,
		},
		{
			title:         "error on first call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 0,
			errorValue:    errors.New("failed retry"),
			expectError:   true,
		},
		{
			title:         "error on second call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 1,
			errorValue:    errors.New("failed retry"),
			expectError:   true,
		},
		{
			title:         "timeout",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 100,
			errorValue:    nil,
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			retries := 0
			err := Retry(tc.timeout, tc.backoff, func() (bool, error) {
				retries++
				if retries < tc.failedRetries {
					return false, nil
				}
				if tc.errorValue != nil {
					return false, tc.errorValue
				}
				return true, nil
			})

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should have failed")
				return
			}
		})
	}
}

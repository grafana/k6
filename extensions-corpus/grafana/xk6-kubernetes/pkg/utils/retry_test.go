package utils

import (
	"fmt"
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
		expectedValue bool
		expectError   bool
	}{
		{
			title:         "Succeed on first call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 0,
			expectedValue: true,
			expectError:   false,
		},
		{
			title:         "Succeed on second call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 1,
			expectedValue: true,
			expectError:   false,
		},
		{
			title:         "error on first call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 0,
			expectedValue: false,
			expectError:   true,
		},
		{
			title:         "error on second call",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 1,
			expectedValue: false,
			expectError:   true,
		},
		{
			title:         "timeout",
			timeout:       time.Second * 5,
			backoff:       time.Second,
			failedRetries: 100,
			expectedValue: false,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			retries := 0
			done, err := Retry(tc.timeout, tc.backoff, func() (bool, error) {
				retries++
				if retries < tc.failedRetries {
					return false, nil
				}
				if tc.expectError {
					return false, fmt.Errorf("Error")
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

			if done != tc.expectedValue {
				t.Errorf("invalid value returned expected %t actual %t", tc.expectedValue, done)
				return
			}
		})
	}
}

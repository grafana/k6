package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// execute a shell and return its pid. On return, the child should had finished
func getDeadProcessPid() string {
	ls := exec.Command("sh", "-c", "cat /proc/self/stat | cut -d' ' -f 1")
	pid, err := ls.Output()
	if err != nil {
		panic("")
	}

	return string(pid)
}

func Test_Acquire(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		createLock  bool
		ownerPid    string
		expectError bool
		acquired    bool
	}{
		{
			title:       "lock does not exist",
			createLock:  false,
			ownerPid:    "",
			expectError: false,
			acquired:    true,
		},
		{
			title:       "lock with empty owner",
			createLock:  true,
			ownerPid:    "",
			expectError: false,
			acquired:    true,
		},
		{
			title:       "process is already owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getpid()),
			expectError: false,
			acquired:    true,
		},
		{
			title:       "lock with other running owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getppid()),
			expectError: false,
			acquired:    false,
		},
		{
			title:       "lock with owner not running",
			createLock:  true,
			ownerPid:    getDeadProcessPid(),
			expectError: false,
			acquired:    true,
		},
	}

	for i, tc := range testCases {
		tmpDir := t.TempDir()

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create file name for test lock file
			testLock := filepath.Join(tmpDir, fmt.Sprintf("test-lockfile.%d", i))
			defer func() {
				_ = os.Remove(testLock)
			}()

			if tc.createLock {
				lockFile, err := os.Create(testLock)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}

				_, err = lockFile.WriteString(tc.ownerPid)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}
			}

			lock := NewFileLock(testLock)

			acquired, err := lock.Acquire()
			if err != nil && !tc.expectError {
				t.Errorf("failed: %v", err)
				return
			}

			if acquired != tc.acquired {
				t.Errorf("expected acquired %t got %t", tc.acquired, acquired)
			}

			if tc.expectError && err == nil {
				t.Errorf("Should had failed")
				return
			}
		})
	}
}

func Test_Release(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		createLock  bool
		ownerPid    string
		expectError bool
	}{
		{
			title:       "process is owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getpid()),
			expectError: false,
		},
		{
			title:       "lock does not exist",
			createLock:  false,
			ownerPid:    "",
			expectError: true,
		},
		{
			title:       "lock with empty owner",
			createLock:  true,
			ownerPid:    "",
			expectError: true,
		},
		{
			title:       "lock with other owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getppid()),
			expectError: true,
		},
	}

	for i, tc := range testCases {
		tmpDir := t.TempDir()

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create file name for test lock file
			testLock := filepath.Join(tmpDir, fmt.Sprintf("test-lockfile.%d", i))
			defer func() {
				_ = os.Remove(testLock)
			}()

			if tc.createLock {
				lockFile, err := os.Create(testLock)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}

				_, err = lockFile.WriteString(tc.ownerPid)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}
			}

			lock := NewFileLock(testLock)

			err := lock.Release()
			if tc.expectError && err == nil {
				t.Errorf("Should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}

func Test_Owner(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title      string
		createLock bool
		ownerPid   string
		expected   int
	}{
		{
			title:      "process is owner",
			createLock: true,
			ownerPid:   fmt.Sprintf("%d", os.Getpid()),
			expected:   os.Getpid(),
		},
		{
			title:      "lock does not exist",
			createLock: false,
			ownerPid:   "",
			expected:   -1,
		},
		{
			title:      "lock with empty owner",
			createLock: true,
			ownerPid:   "",
			expected:   -1,
		},
		{
			title:      "lock with other owner",
			createLock: true,
			ownerPid:   fmt.Sprintf("%d", os.Getppid()),
			expected:   os.Getppid(),
		},
	}

	for i, tc := range testCases {
		tmpDir := t.TempDir()

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create file name for test lock file
			testLock := filepath.Join(tmpDir, fmt.Sprintf("test-lockfile.%d", i))
			defer func() {
				_ = os.Remove(testLock)
			}()

			if tc.createLock {
				lockFile, err := os.Create(testLock)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}

				_, err = lockFile.WriteString(tc.ownerPid)
				if err != nil {
					t.Errorf("error in test setup: %v", err)
					return
				}
			}

			lock := NewFileLock(testLock)

			owner := lock.Owner()
			if owner != tc.expected {
				t.Errorf("expected %d got: %d", tc.expected, owner)
				return
			}
		})
	}
}

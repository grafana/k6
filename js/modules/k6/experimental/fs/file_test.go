package fs

import (
	"errors"
	"testing"
)

func TestFileImpl(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			into     []byte
			fileData []byte
			offset   int
			wantN    int
			wantErr  errorKind
		}{
			{
				name:     "empty file data should fail",
				into:     make([]byte, 10),
				fileData: []byte{},
				offset:   0,
				wantN:    0,
				wantErr:  EOFError,
			},
			{
				name:     "non-empty file data, and empty into should succeed",
				into:     []byte{},
				fileData: []byte("hello"),
				offset:   0,
				wantN:    0,
				wantErr:  0, // No error expected
			},
			{
				name:     "non-empty file data, and non-empty into should succeed",
				into:     make([]byte, 5),
				fileData: []byte("hello"),
				offset:   0,
				wantN:    5,
				wantErr:  0, // No error expected
			},
			{
				name:     "non-empty file data, non-empty into, at an offset should succeed",
				into:     make([]byte, 3),
				fileData: []byte("hello"),
				offset:   1,
				wantN:    3,
				wantErr:  0, // No error expected
			},
			{
				name:     "non-empty file data, non-empty into reading till the end should succeed",
				into:     make([]byte, 5),
				fileData: []byte("hello"),
				offset:   2,
				wantN:    3,
				wantErr:  0, // No error expected
			},
			{
				name:     "non-empty file data, non-empty into with offset at the end should fail",
				into:     make([]byte, 5),
				fileData: []byte("hello"),
				offset:   5,
				wantN:    0,
				wantErr:  EOFError,
			},
		}

		for _, tc := range testCases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				f := &file{
					path:   "",
					data:   tc.fileData,
					offset: tc.offset,
				}

				gotN, err := f.Read(tc.into)

				// Cast the error to your custom error type to access its kind
				var gotErr errorKind
				if err != nil {
					var fsErr *fsError
					ok := errors.As(err, &fsErr)
					if !ok {
						t.Fatalf("unexpected error type: got %T, want %T", err, &fsError{})
					}
					gotErr = fsErr.kind
				}

				if gotN != tc.wantN || gotErr != tc.wantErr {
					t.Errorf("Read() = %d, %v, want %d, %v", gotN, gotErr, tc.wantN, tc.wantErr)
				}
			})
		}
	})
}

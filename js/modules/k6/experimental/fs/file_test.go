package fs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
				wantErr:  EOFError, // No error expected
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

	t.Run("seek", func(t *testing.T) {
		t.Parallel()

		type args struct {
			offset int
			whence SeekMode
		}

		tests := []struct {
			name       string
			fileOffset int
			args       args
			want       int
			wantError  bool
		}{
			{"StartSeekWithinBounds", 0, args{50, SeekModeStart}, 50, false},
			{"StartSeekTooFar", 0, args{150, SeekModeStart}, 0, true},
			{"StartSeekNegative", 0, args{-50, SeekModeStart}, 0, true},
			{"CurrentSeekWithinBounds", 0, args{10, SeekModeCurrent}, 10, false},
			{"CurrentSeekNegativeWithinBounds", 20, args{-20, SeekModeCurrent}, 0, false},
			{"CurrentSeekNegativeTooFar", 20, args{-40, SeekModeCurrent}, 0, true},
			{"CurrentSeekTooFar", 0, args{150, SeekModeCurrent}, 0, true},
			{"EndSeekPositiveOffset", 0, args{20, SeekModeEnd}, 80, true}, // Cannot seek beyond the end of the file
			{"EndSeekWithinBounds", 0, args{-20, SeekModeEnd}, 80, false},
			{"EndSeekTooFar", 0, args{120, SeekModeEnd}, 0, true},
			{"InvalidWhence", 0, args{10, SeekMode(42)}, 0, true},
		}

		for _, tt := range tests {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := &file{data: make([]byte, 100), offset: tt.fileOffset}

				got, err := f.Seek(tt.args.offset, tt.args.whence)
				if tt.wantError {
					assert.Error(t, err, tt.name)
				} else {
					assert.NoError(t, err, tt.name)
					assert.Equal(t, tt.want, got, tt.name)
				}
			})
		}
	})
}

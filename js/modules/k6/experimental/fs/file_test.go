package fs

import (
	"bytes"
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
			wantInto []byte
			wantN    int
			wantErr  errorKind
		}{
			{
				name:     "reading the entire file into a buffer fitting the whole file should succeed",
				into:     make([]byte, 5),
				fileData: []byte("hello"),
				offset:   0,
				wantInto: []byte("hello"),
				wantN:    5,
				wantErr:  0, // No error expected
			},
			{
				name:     "reading a file larger than the provided buffer should succeed",
				into:     make([]byte, 3),
				fileData: []byte("hello"),
				offset:   0,
				wantInto: []byte("hel"),
				wantN:    3,
				wantErr:  0, // No error expected
			},
			{
				name:     "reading a file larger than the provided buffer at an offset should succeed",
				into:     make([]byte, 3),
				fileData: []byte("hello"),
				offset:   2,
				wantInto: []byte("llo"),
				wantN:    3,
				wantErr:  0, // No error expected
			},
			{
				name:     "reading file data into a zero sized buffer should succeed",
				into:     []byte{},
				fileData: []byte("hello"),
				offset:   0,
				wantInto: []byte{},
				wantN:    0,
				wantErr:  0, // No error expected
			},
			{
				name:     "reading past the end of the file should fill the buffer and fail with EOF",
				into:     make([]byte, 10),
				fileData: []byte("hello"),
				offset:   0,
				wantInto: []byte{'h', 'e', 'l', 'l', 'o', 0, 0, 0, 0, 0},
				wantN:    5,
				wantErr:  EOFError,
			},
			{
				name:     "reading into a prefilled buffer overrides its content",
				into:     []byte("world!"),
				fileData: []byte("hello"),
				offset:   0,
				wantInto: []byte("hello!"),
				wantN:    5,
				wantErr:  EOFError,
			},
			{
				name:     "reading an empty file should fail with EOF",
				into:     make([]byte, 10),
				fileData: []byte{},
				offset:   0,
				wantInto: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantN:    0,
				wantErr:  EOFError,
			},
			{
				name: "reading from the end of a file should fail with EOF",
				into: make([]byte, 10),
				// Note that the offset is larger than the file size
				fileData: []byte("hello"),
				offset:   5,
				wantInto: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
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

				if !bytes.Equal(tc.into, tc.wantInto) {
					t.Errorf("Read() into = %v, want %v", tc.into, tc.wantInto)
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

//go:build windows

package fsext_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/lib/fsext"
)

func TestJoinFilePath(t *testing.T) {
	t.Parallel()

	type args struct {
		b string
		p string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "join volume and some folder",
			args: args{
				b: "\\c:",
				p: "test",
			},
			want: "\\c:\\test",
		},
		{
			name: "join volume and some folder with leading slash",
			args: args{
				b: "\\c:",
				p: "\\test",
			},
			want: "\\c:\\test",
		},
		{
			name: "join folder and folder",
			args: args{
				b: "\\c:\\to",
				p: "test",
			},
			want: "\\c:\\to\\test",
		},
		{
			name: "join folder and folder with leading slash",
			args: args{
				b: "\\c:\\to",
				p: "\\test",
			},
			want: "\\c:\\to\\test",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, fsext.JoinFilePath(tt.args.b, tt.args.p))
		})
	}
}

func TestAbs(t *testing.T) {
	t.Parallel()

	type args struct {
		root string
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "absolute path",
			args: args{
				root: "\\",
				path: "\\test",
			},
			want: "\\test",
		},
		{
			name: "relative path",
			args: args{
				root: "\\",
				path: "test",
			},
			want: "\\test",
		},
		{
			name: "relative path with leading dot",
			args: args{
				root: "\\",
				path: ".\\test",
			},
			want: "\\test",
		},
		{
			name: "relative path with leading double dot",
			args: args{
				root: "\\",
				path: "..\\test",
			},
			want: "\\test",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, fsext.Abs(tt.args.root, tt.args.path))
		})
	}
}

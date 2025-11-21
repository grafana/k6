package encoding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

//
// [WPT test]: https://github.com/web-platform-tests/wpt/blob/b5e12f331494f9533ef6211367dace2c88131fd7/encoding/textdecoder-labels.any.js
func TestTextDecoder(t *testing.T) {
	t.Parallel()
	scripts := []testScript{
		{base: "./tests", path: "textdecoder-arguments.js"},
		{base: "./tests", path: "textdecoder-byte-order-marks.js"},
		{base: "./tests", path: "textdecoder-copy.js"},
		{base: "./tests", path: "textdecoder-eof.js"},
		{base: "./tests", path: "textdecoder-fatal.js"},
		{base: "./tests", path: "textdecoder-fatal-streaming.js"},
		{base: "./tests", path: "textdecoder-ignorebom.js"},
		{base: "./tests", path: "textdecoder-labels.js"},
		{base: "./tests", path: "textdecoder-streaming.js"},
		{base: "./tests", path: "textdecoder-utf16-surrogates.js"},
	}

	ts := newTestSetup(t)
	err := executeTestScripts(ts, scripts)
	require.NoError(t, err)
}

func TestTextDecoderUTF8StreamingStateMachine(t *testing.T) {
	t.Parallel()

	t.Run("IncompleteThenInvalidContinuation", func(t *testing.T) {
		td := &TextDecoder{
			TextDecoderCommon: TextDecoderCommon{
				Encoding: UTF8EncodingFormat,
			},
			decoder: unicode.UTF8,
		}

		out, err := td.Decode([]byte{0xF0, 0x9F}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "", out)

		out, err = td.Decode([]byte{0x41}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "\uFFFDA", out)

		out, err = td.Decode(nil, TextDecodeOptions{})
		require.NoError(t, err)
		require.Equal(t, "", out)
	})

	t.Run("ImmediateInvalidStartByte", func(t *testing.T) {
		td := &TextDecoder{
			TextDecoderCommon: TextDecoderCommon{
				Encoding: UTF8EncodingFormat,
			},
			decoder: unicode.UTF8,
		}

		out, err := td.Decode([]byte{0xC1}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "\uFFFD", out)

		out, err = td.Decode(nil, TextDecodeOptions{})
		require.NoError(t, err)
		require.Equal(t, "", out)
	})

	t.Run("ASCIIStreamingProducesOutput", func(t *testing.T) {
		td := &TextDecoder{
			TextDecoderCommon: TextDecoderCommon{
				Encoding: UTF8EncodingFormat,
			},
			decoder: unicode.UTF8,
		}

		out, err := td.Decode([]byte("A"), TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "A", out)

		out, err = td.Decode([]byte("B"), TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "B", out)

		out, err = td.Decode(nil, TextDecodeOptions{})
		require.NoError(t, err)
		require.Equal(t, "", out)
	})

	t.Run("FatalFlushOnTruncatedSequence", func(t *testing.T) {
		td := &TextDecoder{
			TextDecoderCommon: TextDecoderCommon{
				Encoding:  UTF8EncodingFormat,
				Fatal:     true,
				errorMode: FatalErrorMode,
			},
			decoder: unicode.UTF8,
		}

		_, err := td.Decode([]byte{0xF0, 0x9F}, TextDecodeOptions{})
		require.Error(t, err)
	})
}

func TestTextDecoderUTF16FatalStreaming(t *testing.T) {
	t.Parallel()

	newFatalDecoder := func() *TextDecoder {
		return &TextDecoder{
			TextDecoderCommon: TextDecoderCommon{
				Encoding:  UTF16LEEncodingFormat,
				Fatal:     true,
				errorMode: FatalErrorMode,
			},
			decoder: unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
		}
	}

	t.Run("OddFollowedByOddCompletes", func(t *testing.T) {
		td := newFatalDecoder()

		out, err := td.Decode([]byte{0x00}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "", out)

		out, err = td.Decode([]byte{0x00}, TextDecodeOptions{})
		require.NoError(t, err)
		require.Equal(t, "\u0000", out)
	})

	t.Run("EvenThenOddThrows", func(t *testing.T) {
		td := newFatalDecoder()

		out, err := td.Decode([]byte{0x00, 0x00}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "\u0000", out)

		_, err = td.Decode([]byte{0x00}, TextDecodeOptions{})
		require.Error(t, err)
	})

	t.Run("OddThenEvenThrows", func(t *testing.T) {
		td := newFatalDecoder()

		out, err := td.Decode([]byte{0x00}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "", out)

		_, err = td.Decode([]byte{0x00, 0x00}, TextDecodeOptions{})
		require.Error(t, err)
	})

	t.Run("EvenChunksStreamSuccessfully", func(t *testing.T) {
		td := newFatalDecoder()

		out, err := td.Decode([]byte{0x00, 0x00}, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		require.Equal(t, "\u0000", out)

		out, err = td.Decode([]byte{0x00, 0x00}, TextDecodeOptions{})
		require.NoError(t, err)
		require.Equal(t, "\u0000", out)
	})
}

func TestTextDecoderUTF16LEStreamingSingleByteWindow(t *testing.T) {
	t.Parallel()

	encoded := []byte{
		0x00, 0x00, 0x31, 0x00, 0x32, 0x00, 0x33, 0x00,
		0x41, 0x00, 0x42, 0x00, 0x43, 0x00, 0x61, 0x00,
		0x62, 0x00, 0x63, 0x00, 0x80, 0x00, 0xFF, 0x00,
		0x00, 0x01, 0x00, 0x10, 0xFD, 0xFF, 0x00, 0xD8,
		0x00, 0xDC, 0xFF, 0xDB, 0xFF, 0xDF,
	}
	expected := "\x00123ABCabc\u0080\u00FF\u0100\u1000\uFFFD\U00010000\U0010FFFF"

	td := &TextDecoder{
		TextDecoderCommon: TextDecoderCommon{
			Encoding: UTF16LEEncodingFormat,
		},
		decoder: unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	}

	var out strings.Builder
	for _, b := range encoded {
		chunk := []byte{b}
		part, err := td.Decode(chunk, TextDecodeOptions{Stream: true})
		require.NoError(t, err)
		out.WriteString(part)
	}

	part, err := td.Decode(nil, TextDecodeOptions{})
	require.NoError(t, err)
	out.WriteString(part)

	require.Equal(t, expected, out.String())
}

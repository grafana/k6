package zstd

import (
	"fmt"
	"runtime"
	"strings"
)

// DOption is an option for creating a encoder.
type EOption func(*encoderOptions) error

// options retains accumulated state of multiple options.
type encoderOptions struct {
	concurrent int
	crc        bool
	single     *bool
	pad        int
	blockSize  int
	windowSize int
	level      EncoderLevel
}

func (o *encoderOptions) setDefault() {
	*o = encoderOptions{
		// use less ram: true for now, but may change.
		concurrent: runtime.GOMAXPROCS(0),
		crc:        true,
		single:     nil,
		blockSize:  1 << 16,
		windowSize: 1 << 22,
		level:      SpeedDefault,
	}
}

// encoder returns an encoder with the selected options.
func (o encoderOptions) encoder() encoder {
	switch o.level {
	case SpeedDefault:
		return &doubleFastEncoder{fastEncoder: fastEncoder{maxMatchOff: int32(o.windowSize)}}
	case SpeedFastest:
		return &fastEncoder{maxMatchOff: int32(o.windowSize)}
	}
	panic("unknown compression level")
}

// WithEncoderCRC will add CRC value to output.
// Output will be 4 bytes larger.
func WithEncoderCRC(b bool) EOption {
	return func(o *encoderOptions) error { o.crc = b; return nil }
}

// WithEncoderConcurrency will set the concurrency,
// meaning the maximum number of decoders to run concurrently.
// The value supplied must be at least 1.
// By default this will be set to GOMAXPROCS.
func WithEncoderConcurrency(n int) EOption {
	return func(o *encoderOptions) error {
		if n <= 0 {
			return fmt.Errorf("concurrency must be at least 1")
		}
		o.concurrent = n
		return nil
	}
}

// WithEncoderPadding will add padding to all output so the size will be a multiple of n.
// This can be used to obfuscate the exact output size or make blocks of a certain size.
// The contents will be a skippable frame, so it will be invisible by the decoder.
// n must be > 0 and <= 1GB, 1<<30 bytes.
// The padded area will be filled with data from crypto/rand.Reader.
// If `EncodeAll` is used with data already in the destination, the total size will be multiple of this.
func WithEncoderPadding(n int) EOption {
	return func(o *encoderOptions) error {
		if n <= 0 {
			return fmt.Errorf("padding must be at least 1")
		}
		// No need to waste our time.
		if n == 1 {
			o.pad = 0
		}
		if n > 1<<30 {
			return fmt.Errorf("padding must less than 1GB (1<<30 bytes) ")
		}
		o.pad = n
		return nil
	}
}

// EncoderLevel predefines encoder compression levels.
// Only use the constants made available, since the actual mapping
// of these values are very likely to change and your compression could change
// unpredictably when upgrading the library.
type EncoderLevel int

const (
	speedNotSet EncoderLevel = iota

	// SpeedFastest will choose the fastest reasonable compression.
	// This is roughly equivalent to the fastest Zstandard mode.
	SpeedFastest

	// SpeedDefault is the default "pretty fast" compression option.
	// This is roughly equivalent to the default Zstandard mode (level 3).
	SpeedDefault

	// speedLast should be kept as the last actual compression option.
	// The is not for external usage, but is used to keep track of the valid options.
	speedLast

	// SpeedBetterCompression will (in the future) yield better compression than the default,
	// but at approximately 4x the CPU usage of the default.
	// For now this is not implemented.
	SpeedBetterCompression = SpeedDefault

	// SpeedBestCompression will choose the best available compression option.
	// For now this is not implemented.
	SpeedBestCompression = SpeedDefault
)

// EncoderLevelFromString will convert a string representation of an encoding level back
// to a compression level. The compare is not case sensitive.
// If the string wasn't recognized, (false, SpeedDefault) will be returned.
func EncoderLevelFromString(s string) (bool, EncoderLevel) {
	for l := EncoderLevel(speedNotSet + 1); l < speedLast; l++ {
		if strings.EqualFold(s, l.String()) {
			return true, l
		}
	}
	return false, SpeedDefault
}

// EncoderLevelFromZstd will return an encoder level that closest matches the compression
// ratio of a specific zstd compression level.
// Many input values will provide the same compression level.
func EncoderLevelFromZstd(level int) EncoderLevel {
	switch {
	case level < 3:
		return SpeedFastest
	case level >= 3:
		return SpeedDefault
	}
	return SpeedDefault
}

// String provides a string representation of the compression level.
func (e EncoderLevel) String() string {
	switch e {
	case SpeedFastest:
		return "fastest"
	case SpeedDefault:
		return "default"
	default:
		return "invalid"
	}
}

// WithEncoderLevel specifies a predefined compression level.
func WithEncoderLevel(l EncoderLevel) EOption {
	return func(o *encoderOptions) error {
		switch {
		case l <= speedNotSet || l >= speedLast:
			return fmt.Errorf("unknown encoder level")
		}
		o.level = l
		return nil
	}
}

// WithSingleSegment will set the "single segment" flag when EncodeAll is used.
// If this flag is set, data must be regenerated within a single continuous memory segment.
// In this case, Window_Descriptor byte is skipped, but Frame_Content_Size is necessarily present.
// As a consequence, the decoder must allocate a memory segment of size equal or larger than size of your content.
// In order to preserve the decoder from unreasonable memory requirements,
// a decoder is allowed to reject a compressed frame which requests a memory size beyond decoder's authorized range.
// For broader compatibility, decoders are recommended to support memory sizes of at least 8 MB.
// This is only a recommendation, each decoder is free to support higher or lower limits, depending on local limitations.
// If this is not specified, block encodes will automatically choose this based on the input size.
// This setting has no effect on streamed encodes.
func WithSingleSegment(b bool) EOption {
	return func(o *encoderOptions) error {
		o.single = &b
		return nil
	}
}

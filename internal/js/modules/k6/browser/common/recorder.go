package common

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// Recorder captures CDP screencast frames for a single page and pipes them
// into ffmpeg to produce a webm video. Methods are safe for concurrent use.
type Recorder struct {
	logger  *log.Logger
	outPath string

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stderr  *bytes.Buffer
	closed  bool
	broken  bool
	frames  int
}

// NewRecorder creates the output directory and spawns an ffmpeg process that
// reads JPEG frames from stdin and writes a webm to <dir>/<name>.webm.
//
// Pre-condition: ffmpeg must be on PATH. Callers should fall back gracefully
// when this returns an error.
func NewRecorder(dir, name string, framerate int, logger *log.Logger) (*Recorder, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found on PATH: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating recording dir %q: %w", dir, err)
	}
	if framerate <= 0 {
		framerate = 10
	}

	outPath := filepath.Join(dir, name+".webm")
	// libvpx (VP8) is broadly supported by ffmpeg builds. The pad filter ensures
	// even dimensions, which the encoder requires.
	cmd := exec.Command("ffmpeg", //nolint:gosec
		"-y",
		"-loglevel", "error",
		"-f", "image2pipe",
		"-framerate", strconv.Itoa(framerate),
		"-i", "-",
		"-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2",
		"-c:v", "libvpx",
		"-b:v", "1M",
		outPath,
	)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("getting ffmpeg stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	return &Recorder{
		logger:  logger,
		outPath: outPath,
		cmd:     cmd,
		stdin:   stdin,
		stderr:  stderr,
	}, nil
}

// WriteFrame decodes a base64 JPEG screencast frame and forwards it to ffmpeg.
// Frames received after Close, or after the ffmpeg pipe has broken once, are
// silently dropped — the first pipe failure is reported, then we latch off to
// avoid log spam at the screencast frame rate.
func (r *Recorder) WriteFrame(b64 string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.broken {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("decoding screencast frame: %w", err)
	}
	if _, err := r.stdin.Write(data); err != nil {
		r.broken = true
		return fmt.Errorf("writing frame to ffmpeg: %w", err)
	}
	r.frames++
	return nil
}

// Close finalizes the recording: it closes ffmpeg's stdin and waits for the
// process to exit. Safe to call multiple times.
func (r *Recorder) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	stdin := r.stdin
	cmd := r.cmd
	frames := r.frames
	out := r.outPath
	stderr := r.stderr
	r.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil {
		return nil
	}

	if frames == 0 {
		// No frames captured; kill ffmpeg to avoid it waiting forever and
		// remove the empty output file it may have created.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(out)
		r.logger.Warnf("Recorder", "no frames captured; recording skipped")
		return nil
	}

	if err := cmd.Wait(); err != nil {
		// ExitError still produces a (possibly partial) file. Surface as warning
		// rather than failing the test.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.logger.Warnf("Recorder", "ffmpeg exited with %v; output may be partial: %s; stderr: %s",
				err, out, strings.TrimSpace(stderr.String()))
			return nil
		}
		return fmt.Errorf("waiting for ffmpeg: %w", err)
	}

	return nil
}

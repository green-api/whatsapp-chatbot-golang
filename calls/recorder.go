package calls

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Track string

const (
	TrackCaller Track = "caller"
	TrackBot    Track = "bot"
)

type captureEntry struct {
	track Track
	chunk PcmChunk
}

type PcmCapture struct {
	mu                 sync.Mutex
	options            RecordingOptions
	queue              chan captureEntry
	done               chan struct{}
	tracks             map[Track]*os.File
	err                error
	closed, hasSamples bool
}

func NewPcmCapture(o RecordingOptions) *PcmCapture {
	return &PcmCapture{options: o, tracks: map[Track]*os.File{}}
}

func (c *PcmCapture) start() error {
	if c.queue != nil {
		return nil
	}

	for _, t := range []Track{TrackCaller, TrackBot} {
		f, e := os.CreateTemp(c.options.Directory, "call-pcm-*")

		if e != nil {
			for _, f := range c.tracks {
				f.Close()
				os.Remove(f.Name())
			}

			c.tracks = map[Track]*os.File{}

			return e
		}

		c.tracks[t] = f
	}

	c.queue = make(chan captureEntry, c.options.MaxPendingChunks)
	c.done = make(chan struct{})

	go c.writeLoop()

	return nil
}

func (c *PcmCapture) Write(t Track, ch PcmChunk) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("PCM capture is closed")
	}

	if c.err != nil {
		return c.err
	}

	if ch.Position < 0 || len(ch.Data)%2 != 0 {
		return errors.New("invalid PCM chunk")
	}

	if t != TrackCaller && t != TrackBot {
		return errors.New("unknown track")
	}

	if len(ch.Data) == 0 {
		return nil
	}

	if e := c.start(); e != nil {
		return e
	}

	ch.Data = append([]byte(nil), ch.Data...)

	select {
	case c.queue <- captureEntry{t, ch}:
		c.hasSamples = true

		return nil
	default:
		return errors.New("recording writer cannot keep up")
	}
}

func (c *PcmCapture) writeLoop() {
	defer close(c.done)

	for entry := range c.queue {
		f := c.tracks[entry.track]
		_, e := f.WriteAt(entry.chunk.Data, entry.chunk.Position*2)

		if e != nil {
			c.mu.Lock()
			c.err = e
			c.mu.Unlock()

			for range c.queue {
			}

			return
		}
	}
}

func (c *PcmCapture) Seal() (map[Track]*os.File, error) {
	c.mu.Lock()

	if !c.closed {
		c.closed = true

		if c.queue != nil {
			close(c.queue)
		}
	}

	done := c.done
	c.mu.Unlock()

	if done != nil {
		<-done
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.tracks, c.err
}

func (c *PcmCapture) Close() error {
	_, err := c.Seal()

	for _, f := range c.tracks {
		err = errors.Join(err, f.Close())
		e := os.Remove(f.Name())

		if !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}

	return err
}

type Mp3Encoder struct{ Options RecordingOptions }

// RecordingError reports a recording failure without exposing file paths in Error.
// Diagnostic contains bounded encoder output for explicit inspection.
type RecordingError struct {
	Stage      string
	Cause      error
	Diagnostic string
}

func (e *RecordingError) Error() string {
	var exit *exec.ExitError
	if errors.As(e.Cause, &exit) {
		return fmt.Sprintf("recording %s failed (exit %d)", e.Stage, exit.ExitCode())
	}
	return "recording " + e.Stage + " failed"
}
func (e *RecordingError) Unwrap() error { return e.Cause }

type encoderDiagnostics struct{ data []byte }

func (d *encoderDiagnostics) Write(p []byte) (int, error) {
	const limit = 4096
	n := len(p)
	d.data = append(d.data, p[:min(len(p), limit-len(d.data))]...)
	return n, nil
}

// Encode streams saturated PCM16 mono mixing to FFmpeg/libmp3lame.
func (e Mp3Encoder) Encode(ctx context.Context, tracks map[Track]*os.File, path string) error {
	bin := e.Options.EncoderPath

	if bin == "" {
		bin = "ffmpeg"
	}

	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-loglevel", "error", "-f", "s16le", "-ar", "24000", "-ac", "1", "-i", "pipe:0", "-codec:a", "libmp3lame", "-b:a", strconv.Itoa(e.Options.BitrateBPS), "-y", path)
	diagnostic := &encoderDiagnostics{}
	cmd.Stderr = diagnostic
	in, err := cmd.StdinPipe()

	if err != nil {
		return &RecordingError{Stage: "encoder_input", Cause: err}
	}

	if err := cmd.Start(); err != nil {
		_ = in.Close()
		return &RecordingError{Stage: "encoder_start", Cause: err}
	}

	writeErr := e.writeTracks(in, tracks)
	closeErr := in.Close()
	waitErr := cmd.Wait()

	if err := errors.Join(writeErr, closeErr, waitErr, ctx.Err()); err != nil {
		detail := strings.ReplaceAll(string(diagnostic.data), path, "[recording]")
		return &RecordingError{Stage: "encode", Cause: err, Diagnostic: strings.TrimSpace(detail)}
	}

	return nil
}

func (e Mp3Encoder) writeTracks(out io.Writer, tracks map[Track]*os.File) error {
	if e.Options.ExportBlockSamples <= 0 {
		return errors.New("export block size must be positive")
	}

	files := []*os.File{tracks[TrackCaller], tracks[TrackBot]}
	bufs := [][]byte{make([]byte, e.Options.ExportBlockSamples*2), make([]byte, e.Options.ExportBlockSamples*2)}

	for _, f := range files {
		if f != nil {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
	}

	for {
		n, err := readRecordingBlock(files, bufs)

		if err != nil {
			return err
		}

		if n == 0 {
			return nil
		}

		if _, err := out.Write(mixRecordingBlock(bufs, n)); err != nil {
			return err
		}
	}
}

func readRecordingBlock(files []*os.File, bufs [][]byte) (int, error) {
	size := 0

	for i, f := range files {
		clear(bufs[i])

		if f == nil {
			continue
		}

		n, err := io.ReadFull(f, bufs[i])

		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, err
		}

		size = max(size, n)
	}

	return size, nil
}

func mixRecordingBlock(bufs [][]byte, n int) []byte {
	mixed := make([]byte, n)

	for j := 0; j+1 < n; j += 2 {
		a := int(int16(binary.LittleEndian.Uint16(bufs[0][j:])))
		b := int(int16(binary.LittleEndian.Uint16(bufs[1][j:])))
		binary.LittleEndian.PutUint16(mixed[j:], uint16(int16(max(-32768, min(32767, a+b)))))
	}

	return mixed
}

type CallRecorder struct {
	mu                  sync.Mutex
	options             RecordingOptions
	capture             *PcmCapture
	disabled, discarded bool
	started             bool
	done                chan struct{}
	path                string
	err                 error
}

func NewCallRecorder(o RecordingOptions) *CallRecorder {
	return &CallRecorder{options: o, capture: NewPcmCapture(o)}
}

func (r *CallRecorder) Write(t Track, ch PcmChunk) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.options.Enabled || r.disabled || r.discarded {
		return nil
	}

	return r.capture.Write(t, ch)
}

func (r *CallRecorder) Disable(err error) {
	r.mu.Lock()
	r.disabled = true
	r.err = errors.Join(r.err, err)
	r.mu.Unlock()

	var recording *RecordingError

	if errors.As(err, &recording) {
		slog.Warn("Call recording disabled", "stage", recording.Stage, "error", recording.Error(), "diagnostic", recording.Diagnostic)
		return
	}

	slog.Warn("Call recording disabled", "error_type", fmt.Sprintf("%T", err))
}

func (r *CallRecorder) failure() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *CallRecorder) Finish(ctx context.Context) (string, error) {
	r.mu.Lock()

	if !r.started {
		r.started = true
		r.done = make(chan struct{})

		go r.finishBlocking()
	}

	done := r.done
	r.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
		r.mu.Lock()
		defer r.mu.Unlock()

		return r.path, r.err
	}
}

func (r *CallRecorder) finishBlocking() {
	defer close(r.done)

	defer func() {
		if err := r.capture.Close(); err != nil {
			r.Disable(&RecordingError{Stage: "capture_close", Cause: err})
		}
	}()

	tracks, err := r.capture.Seal()

	if err != nil {
		r.Disable(&RecordingError{Stage: "capture_seal", Cause: err})
		return
	}

	r.mu.Lock()
	skip := r.disabled || r.discarded || !r.options.Enabled
	r.mu.Unlock()

	if skip || !r.capture.hasSamples {
		return
	}

	path, err := r.export(tracks)

	if err != nil {
		r.Disable(err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.discarded {
		_ = os.Remove(path)
	} else {
		r.path = path
	}
}

func (r *CallRecorder) export(tracks map[Track]*os.File) (string, error) {
	f, err := os.CreateTemp(r.options.Directory, "call-*.mp3")

	if err != nil {
		return "", &RecordingError{Stage: "output_create", Cause: err}
	}

	path := f.Name()
	err = f.Close()

	if err == nil {
		err = (Mp3Encoder{r.options}).Encode(context.Background(), tracks, path)
	}

	if err != nil {
		removeErr := os.Remove(path)
		return "", errors.Join(err, removeErr)
	}

	return path, nil
}

func (r *CallRecorder) TakeReadyOrDiscard() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.path != "" {
		p := r.path
		r.path = ""

		return p
	}

	r.discarded = true

	return ""
}

func (r *CallRecorder) ExportRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.done == nil {
		return false
	}

	select {
	case <-r.done:
		return false
	default:
		return true
	}
}

func (r *CallRecorder) Discard() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discarded = true

	if r.path != "" {
		os.Remove(r.path)
		r.path = ""
	}
}

func (r *CallRecorder) Close(ctx context.Context) error {
	r.Discard()
	_, err := r.Finish(ctx)

	return err
}

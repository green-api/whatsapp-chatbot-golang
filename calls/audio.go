package calls

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"
	"time"
	// TODO: Remove this local import override in go.mod after the VoIP SDK release.
	"github.com/green-api/whatsapp-api-client-golang-v2/opus"
	sdk "github.com/green-api/whatsapp-api-client-golang-v2/voip"
)

type AudioSource interface {
	ReadFrame() []byte
	ReplaceTrack()
}

type AudioSink interface {
	WriteAudio(context.Context, []byte) error
}

type InputAudioBuffer struct {
	mu             sync.Mutex
	options        InputAudioOptions
	frames         [][]byte
	bytes          int
	closed         bool
	changed        chan struct{}
	DroppedSamples int64
}

func NewInputAudioBuffer(o InputAudioOptions) *InputAudioBuffer {
	return &InputAudioBuffer{options: o, changed: make(chan struct{})}
}

func (b *InputAudioBuffer) signal() { close(b.changed); b.changed = make(chan struct{}) }

func (b *InputAudioBuffer) Put(ctx context.Context, data []byte) error {
	if len(data)%2 != 0 {
		return errors.New("invalid PCM samples")
	}

	for len(data) > 0 {
		n := min(len(data), PCMBytesPerFrame)

		if err := b.putFrame(ctx, data[:n]); err != nil {
			return err
		}

		data = data[n:]
	}

	return nil
}

func (b *InputAudioBuffer) putFrame(ctx context.Context, data []byte) error {
	deadline := time.Now().Add(seconds(b.options.EnqueueSeconds))
	capacity := max(PCMBytesPerFrame, int(b.options.BufferSeconds*PCMRate)*2)

	for {
		b.mu.Lock()

		if b.closed {
			b.mu.Unlock()

			return errors.New("input buffer is closed")
		}

		if b.bytes+len(data) <= capacity {
			b.frames = append(b.frames, append([]byte(nil), data...))
			b.bytes += len(data)
			b.signal()
			b.mu.Unlock()

			return nil
		}

		if b.options.OverflowPolicy == InputOverflowPolicyDropOldest {
			for b.bytes+len(data) > capacity && len(b.frames) > 0 {
				f := b.frames[0]
				b.frames[0] = nil
				b.frames = b.frames[1:]
				b.bytes -= len(f)
				b.DroppedSamples += int64(len(f) / 2)
			}

			b.mu.Unlock()
			continue
		}

		wake := b.changed
		b.mu.Unlock()
		timer := time.NewTimer(time.Until(deadline))

		select {
		case <-ctx.Done():
			timer.Stop()

			return ctx.Err()
		case <-timer.C:
			return errors.New("input audio buffer overflow")
		case <-wake:
			timer.Stop()
		}
	}
}

func (b *InputAudioBuffer) Get(ctx context.Context) ([]byte, error) {
	for {
		b.mu.Lock()

		if len(b.frames) > 0 {
			f := b.frames[0]
			b.frames[0] = nil
			b.frames = b.frames[1:]
			b.bytes -= len(f)
			b.signal()
			b.mu.Unlock()

			return f, nil
		}

		if b.closed {
			b.mu.Unlock()

			return nil, io.EOF
		}

		wake := b.changed
		b.mu.Unlock()

		select {
		case <-wake:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (b *InputAudioBuffer) Finish() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.closed {
		b.closed = true
		b.signal()
	}
}

func (b *InputAudioBuffer) Dropped() int64 { b.mu.Lock(); defer b.mu.Unlock(); return b.DroppedSamples }

type RecordingTap struct {
	mu                     sync.Mutex
	recorder               *CallRecorder
	epoch                  time.Time
	positions              map[Track]int64
	originMedia, lastMedia float64
	originPosition         int64
	hasOrigin, hasLast     bool
}

func NewRecordingTap(r *CallRecorder) *RecordingTap {
	return &RecordingTap{recorder: r, positions: map[Track]int64{}}
}

func (t *RecordingTap) Activate() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.epoch.IsZero() {
		t.epoch = time.Now()
	}
}

func (t *RecordingTap) Write(track Track, data []byte, media *float64, at time.Time, resumed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.epoch.IsZero() {
		return
	}

	if at.IsZero() {
		at = time.Now()
	}

	wall := max(0, int64(math.Round(at.Sub(t.epoch).Seconds()*PCMRate)))
	next, ok := t.positions[track]
	pos := next

	if !ok || (track == TrackBot && resumed) {
		pos = max(next, wall)
	}

	if track == TrackCaller {
		if media != nil && !math.IsNaN(*media) && !math.IsInf(*media, 0) {
			elapsed := *media - t.originMedia
			restarted := t.hasLast && *media < t.lastMedia-.02
			allowed := float64(max(pos, wall)+PCMRate-t.originPosition) / PCMRate

			if t.hasOrigin && !restarted && elapsed >= -.02 && elapsed <= allowed {
				pos = max(pos, t.originPosition+int64(math.Round(elapsed*PCMRate)))
			} else {
				pos = max(pos, wall)
				t.originMedia = *media
				t.originPosition = pos
				t.hasOrigin = true
			}

			t.lastMedia = *media
			t.hasLast = true
		} else {
			t.hasOrigin = false
			t.hasLast = false

			if wall > pos+2*PCMSamplesPerFrame {
				pos = wall
			}
		}
	}

	if err := t.recorder.Write(track, PcmChunk{pos, data}); err != nil {
		t.recorder.Disable(err)

		return
	}

	t.positions[track] = pos + int64(len(data)/2)
}

func (t *RecordingTap) ReplaceTracks() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.positions = map[Track]int64{}
	t.hasOrigin = false
	t.hasLast = false
}

type BotTrack struct {
	mu        sync.Mutex
	source    AudioSource
	tap       *RecordingTap
	emit      EventSink
	epoch     time.Time
	position  int64
	resampler pcmRateConverter
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewBotTrack(source AudioSource, tap *RecordingTap, emit EventSink) *BotTrack {
	ctx, cancel := context.WithCancel(context.Background())

	return &BotTrack{source: source, tap: tap, emit: emit, epoch: time.Now(), ctx: ctx, cancel: cancel}
}

func (t *BotTrack) Recv(ctx context.Context) (sdk.AudioFrame, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	due := t.epoch.Add(time.Duration(t.position) * time.Second / PCMRate)
	timer := time.NewTimer(max(0, time.Until(due)))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return sdk.AudioFrame{}, ctx.Err()
	case <-t.ctx.Done():
		return sdk.AudioFrame{}, io.EOF
	case <-timer.C:
	}

	data := t.source.ReadFrame()
	at := time.Now()

	if len(data) != PCMBytesPerFrame {
		err := errors.New("AudioSource must return exactly one PCM frame")
		t.emit(RuntimeEvent{Kind: "media_failed", Error: err})

		return sdk.AudioFrame{}, err
	}

	resumed := at.After(due.Add(PCMFrameDuration))

	if resumed {
		t.position = max(t.position, int64(math.Round(at.Sub(t.epoch).Seconds()*PCMRate)))
	}

	t.tap.Write(TrackBot, data, nil, at, resumed)
	samples := t.resampler.upsample(data)

	frame := sdk.AudioFrame{Samples: samples, SampleRate: 48000, Channels: 1, PTS: t.position * 2}
	t.position += 480

	return frame, nil
}

func (t *BotTrack) Close() error { t.cancel(); return nil }

type CallerSink struct {
	mu      sync.Mutex
	sink    AudioSink
	tap     *RecordingTap
	emit    EventSink
	options InputAudioOptions
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
}

func NewCallerSink(s AudioSink, t *RecordingTap, e EventSink, o InputAudioOptions) *CallerSink {
	ctx, c := context.WithCancel(context.Background())

	return &CallerSink{sink: s, tap: t, emit: e, options: o, ctx: ctx, cancel: c}
}

func (s *CallerSink) Attach(ctx context.Context, track sdk.RemoteAudioTrack) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("caller audio sink is closing")
	}

	s.wg.Add(1)

	go s.consume(track)

	return nil
}

func (s *CallerSink) consume(track sdk.RemoteAudioTrack) {
	defer s.wg.Done()
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	resampler := pcmRateConverter{}
	b := NewInputAudioBuffer(s.options)
	done := make(chan error, 1)

	go func() {
		var err error

		defer func() { done <- err; cancel() }()

		for {
			var data []byte
			data, err = b.Get(ctx)

			if err != nil {
				if err == io.EOF {
					err = nil
				}

				return
			}

			sendCtx, c := context.WithTimeout(ctx, seconds(s.options.SendSeconds))
			err = s.sink.WriteAudio(sendCtx, data)
			c()

			if err != nil {
				return
			}
		}
	}()

	var readErr error
	var reported int64
	nextReport := time.Time{}

	for {
		frame, err := track.Recv(ctx)

		if err != nil {
			readErr = err
			break
		}

		if frame.SampleRate != 48000 && frame.SampleRate != 24000 || frame.Channels < 1 || frame.Channels > 2 || len(frame.Samples)%frame.Channels != 0 {
			readErr = errors.New("unsupported RTC PCM format")
			break
		}

		var data []byte

		if frame.SampleRate == 48000 {
			data = resampler.downsample(frame.Samples, frame.Channels)
		} else {
			data = monoPCM(frame.Samples, frame.Channels)
		}

		media := float64(frame.PTS) / float64(frame.SampleRate)
		at := time.Now().Add(-time.Duration(len(data)/2) * time.Second / PCMRate)
		s.tap.Write(TrackCaller, data, &media, at, false)

		if err = b.Put(ctx, data); err != nil {
			readErr = err
			break
		}

		if dropped := b.Dropped(); dropped > reported && time.Now().After(nextReport) {
			s.emit(RuntimeEvent{Kind: "audio_loss", Loss: AudioLoss{Direction: "caller", DroppedSamples: dropped - reported}})
			reported = dropped
			nextReport = time.Now().Add(time.Second)
		}
	}

	b.Finish()

	if readErr != io.EOF {
		cancel()
	}

	sendErr := <-done

	if n := b.Dropped(); n > reported {
		s.emit(RuntimeEvent{Kind: "audio_loss", Loss: AudioLoss{Direction: "caller", DroppedSamples: n - reported}})
	}

	if s.ctx.Err() == nil {
		if readErr != nil && readErr != io.EOF && !errors.Is(readErr, context.Canceled) {
			s.emit(RuntimeEvent{Kind: "media_failed", Error: readErr})
		} else if sendErr != nil && !errors.Is(sendErr, context.Canceled) {
			s.emit(RuntimeEvent{Kind: "media_failed", Error: sendErr})
		}
	}
}

func (s *CallerSink) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()

	return nil
}

type AudioBridge struct {
	mu      sync.Mutex
	source  AudioSource
	sink    AudioSink
	tap     *RecordingTap
	emit    EventSink
	options InputAudioOptions
	track   *BotTrack
	caller  *CallerSink
	closed  bool
}

func NewAudioBridge(source AudioSource, sink AudioSink, r *CallRecorder, emit EventSink, o InputAudioOptions) *AudioBridge {
	return &AudioBridge{source: source, sink: sink, tap: NewRecordingTap(r), emit: emit, options: o}
}

func (b *AudioBridge) ActivateRecording() { b.tap.Activate() }

func (b *AudioBridge) Create(ctx context.Context) (*sdk.CallAudio, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, errors.New("audio bridge is closing")
	}

	if b.track != nil {
		b.track.Close()
		b.caller.Close()
		b.source.ReplaceTrack()
		b.tap.ReplaceTracks()
	}

	b.track = NewBotTrack(b.source, b.tap, b.emit)
	b.caller = NewCallerSink(b.sink, b.tap, b.emit, b.options)
	track, caller := b.track, b.caller

	return &sdk.CallAudio{LocalTrack: sdk.NewPCMTrack(track, opus.New, 1), OnRemoteTrack: caller.Attach, Close: func() error { return errors.Join(track.Close(), caller.Close()) }}, nil
}

func (b *AudioBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true

	if b.track == nil {
		return nil
	}

	return errors.Join(b.track.Close(), b.caller.Close())
}

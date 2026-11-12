package calls

import (
	"context"
	"encoding/binary"
	"errors"
	sdk "github.com/green-api/whatsapp-api-client-golang-v2/voip"
	"io"
	"testing"
	"time"
)

func TestInputOverflowKeepsRecentAudio(t *testing.T) {
	o := DefaultInputAudioOptions()
	o.BufferSeconds = .02
	b := NewInputAudioBuffer(o)
	old := make([]byte, 960)
	recent := make([]byte, 960)
	recent[0] = 42
	b.Put(context.Background(), old)
	b.Put(context.Background(), recent)
	got, _ := b.Get(context.Background())

	if got[0] != 42 || b.Dropped() != 480 {
		t.Fatal("drop_oldest mismatch")
	}
}

func TestFinishingInputBufferWakesWaiters(t *testing.T) {
	b := NewInputAudioBuffer(DefaultInputAudioOptions())
	done := make(chan error, 1)

	go func() { _, err := b.Get(context.Background()); done <- err }()

	b.Finish()

	if err := <-done; err != io.EOF {
		t.Fatal(err)
	}

	if b.Put(context.Background(), make([]byte, 2)) == nil {
		t.Fatal("closed buffer reopened")
	}
}

func TestInputFailPolicyIsBounded(t *testing.T) {
	o := DefaultInputAudioOptions()
	o.OverflowPolicy = InputOverflowPolicyFail
	o.BufferSeconds = .02
	o.EnqueueSeconds = .001
	b := NewInputAudioBuffer(o)
	b.Put(context.Background(), make([]byte, 960))

	if b.Put(context.Background(), make([]byte, 960)) == nil {
		t.Fatal("overflow accepted")
	}
}

type testSource struct{ data []byte }

func (s testSource) ReadFrame() []byte { return s.data }

func (s testSource) ReplaceTrack() {}

func TestBotFramesHave20msTimestampsAndBadSourceFails(t *testing.T) {
	o := DefaultRecordingOptions()
	o.Enabled = false
	tap := NewRecordingTap(NewCallRecorder(o))
	source := testSource{make([]byte, 960)}
	events := NewEvents()
	track := NewBotTrack(source, tap, events.Emit)
	defer track.Close()
	a, err := track.Recv(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	b, err := track.Recv(context.Background())

	if err != nil || b.PTS-a.PTS != 960 || a.SampleRate != 48000 || len(a.Samples) != 960 {
		t.Fatal("frame timing")
	}

	bad := NewBotTrack(testSource{[]byte{1}}, tap, events.Emit)
	defer bad.Close()

	if _, err := bad.Recv(context.Background()); err == nil {
		t.Fatal("bad source accepted")
	}
}

type blockingRemote struct{}

func (blockingRemote) Recv(ctx context.Context) (sdk.AudioFrame, error) {
	<-ctx.Done()

	return sdk.AudioFrame{}, ctx.Err()
}

type oneRemote struct{ sent bool }

func (r *oneRemote) Recv(ctx context.Context) (sdk.AudioFrame, error) {
	if r.sent {
		<-ctx.Done()

		return sdk.AudioFrame{}, ctx.Err()
	}

	r.sent = true

	return sdk.AudioFrame{Samples: make([]int16, 960), SampleRate: 48000, Channels: 1}, nil
}

type brokenAudioSink struct{}

func (brokenAudioSink) WriteAudio(context.Context, []byte) error { return errors.New("send failed") }

func TestSenderFailureReportedWithoutAnotherRTCFrame(t *testing.T) {
	o := DefaultRecordingOptions()
	o.Enabled = false
	events := NewEvents()
	sink := NewCallerSink(brokenAudioSink{}, NewRecordingTap(NewCallRecorder(o)), events.Emit, DefaultInputAudioOptions())
	sink.Attach(context.Background(), &oneRemote{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	event, err := events.Receive(ctx, NewDeadlines())
	sink.Close()

	if err != nil || event.Kind != "media_failed" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
}

func TestRecordingTapMediaGapAndTimestampRestart(t *testing.T) {
	o := DefaultRecordingOptions()
	o.Directory = t.TempDir()
	r := NewCallRecorder(o)
	tap := NewRecordingTap(r)
	tap.Activate()
	at := time.Now()
	m := 1.
	tap.Write(TrackCaller, make([]byte, 960), &m, at, false)
	m = 1.1
	tap.Write(TrackCaller, make([]byte, 960), &m, at.Add(100*time.Millisecond), false)
	m = 0
	tap.Write(TrackCaller, make([]byte, 960), &m, at.Add(120*time.Millisecond), false)

	if tap.positions[TrackCaller] < 3300 {
		t.Fatal("gap/reanchor lost")
	}

	r.Close(context.Background())
}

func TestRecordingTapMixSourceSampleConversion(t *testing.T) {
	data := make([]byte, 960)
	for i := 0; i < len(data); i += 2 {
		binary.LittleEndian.PutUint16(data[i:], 1234)
	}
	track := NewBotTrack(testSource{data}, NewRecordingTap(NewCallRecorder(DefaultRecordingOptions())), func(RuntimeEvent) bool { return true })
	defer track.Close()
	var frame sdk.AudioFrame
	var err error

	for i := 0; i < 3; i++ {
		frame, err = track.Recv(context.Background())
	}

	if err != nil || frame.Samples[100] < 1200 || frame.Samples[100] > 1270 || frame.Samples[101] < 1200 || frame.Samples[101] > 1270 {
		t.Fatal("24k to 48k conversion")
	}
}

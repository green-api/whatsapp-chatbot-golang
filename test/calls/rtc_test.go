package calls_test

import (
	"context"
	"encoding/binary"
	"github.com/green-api/whatsapp-api-client-golang-v2/opus"
	sdk "github.com/green-api/whatsapp-api-client-golang-v2/voip"
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

type toneVoice struct {
	index    int
	received atomic.Int32
}

func (s *toneVoice) ReadFrame() []byte {
	data := make([]byte, 960)

	for i := 0; i < 480; i++ {
		sample := int16(10000 * math.Sin(2*math.Pi*440*float64(s.index)/24000))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(sample))
		s.index++
	}

	return data
}

func (s *toneVoice) ReplaceTrack() {}

func (s *toneVoice) WriteAudio(_ context.Context, data []byte) error {
	for i := 0; i+1 < len(data); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(data[i:]))

		if sample > 1000 || sample < -1000 {
			s.received.Add(1)
			break
		}
	}

	return nil
}

func TestRealRTCBridgeExchangesDecodedBotAndCallerAudio(t *testing.T) {
	if _, err := opus.New(1); err != nil {
		t.Skip(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	left, err := sdk.NewRTCAdapter(nil)

	if err != nil {
		t.Fatal(err)
	}

	defer left.Close()
	right, err := sdk.NewRTCAdapter(nil)

	if err != nil {
		t.Fatal(err)
	}

	defer right.Close()
	o := calls.DefaultRecordingOptions()
	o.Enabled = false
	voice, otherVoice := &toneVoice{}, &toneVoice{}
	audio := calls.NewAudioBridge(voice, voice, calls.NewCallRecorder(o), func(calls.RuntimeEvent) bool { return true }, calls.DefaultInputAudioOptions())
	defer audio.Close()
	otherAudio := calls.NewAudioBridge(otherVoice, otherVoice, calls.NewCallRecorder(o), func(calls.RuntimeEvent) bool { return true }, calls.DefaultInputAudioOptions())
	defer otherAudio.Close()
	tracks, err := audio.Create(ctx)

	if err != nil {
		t.Fatal(err)
	}

	otherTracks, err := otherAudio.Create(ctx)

	if err != nil {
		t.Fatal(err)
	}

	if err = left.AddTrack(tracks.LocalTrack); err != nil {
		t.Fatal(err)
	}

	if err = right.AddTrack(otherTracks.LocalTrack); err != nil {
		t.Fatal(err)
	}

	left.OnTrack(func(track sdk.RemoteAudioTrack) {
		if err := tracks.OnRemoteTrack(ctx, track); err != nil {
			t.Error(err)
		}
	})
	right.OnTrack(func(track sdk.RemoteAudioTrack) {
		if err := otherTracks.OnRemoteTrack(ctx, track); err != nil {
			t.Error(err)
		}
	})
	offer, err := left.CreateOffer()

	if err != nil {
		t.Fatal(err)
	}

	if err = left.SetLocalDescription(ctx, offer); err != nil {
		t.Fatal(err)
	}

	if err = right.SetRemoteDescription(*left.LocalDescription()); err != nil {
		t.Fatal(err)
	}

	answer, err := right.CreateAnswer()

	if err != nil {
		t.Fatal(err)
	}

	if err = right.SetLocalDescription(ctx, answer); err != nil {
		t.Fatal(err)
	}

	if err = left.SetRemoteDescription(*right.LocalDescription()); err != nil {
		t.Fatal(err)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for voice.received.Load() < 5 || otherVoice.received.Load() < 5 {
		select {
		case <-ctx.Done():
			t.Fatalf("decoded audio not received both ways: %d/%d %v", voice.received.Load(), otherVoice.received.Load(), ctx.Err())
		case <-ticker.C:
		}
	}
}

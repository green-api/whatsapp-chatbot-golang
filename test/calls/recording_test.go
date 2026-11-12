package calls_test

import (
	"context"
	"encoding/binary"
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRealMp3ContainsBothSidesAndCallerOwnsOutput(t *testing.T) {
	encoder := os.Getenv("CALLS_TEST_FFMPEG")

	if encoder == "" {
		encoder = "ffmpeg"
	}

	if _, err := exec.LookPath(encoder); err != nil {
		if os.Getenv("CALLS_REQUIRE_FFMPEG") == "1" {
			t.Fatalf("FFmpeg required for recording integration: %v", err)
		}

		t.Skip("FFmpeg unavailable; set CALLS_TEST_FFMPEG")
	}

	o := calls.DefaultRecordingOptions()
	o.EncoderPath = encoder
	o.Directory = t.TempDir()
	r := calls.NewCallRecorder(o)
	a := make([]byte, 24000)
	b := make([]byte, 24000)

	for i := 0; i < len(a); i += 2 {
		binary.LittleEndian.PutUint16(a[i:], uint16(int16(6000*math.Sin(2*math.Pi*440*float64(i/2)/24000))))
		binary.LittleEndian.PutUint16(b[i:], uint16(int16(6000*math.Sin(2*math.Pi*880*float64(i/2)/24000))))
	}

	r.Write(calls.TrackCaller, calls.PcmChunk{0, a})
	r.Write(calls.TrackBot, calls.PcmChunk{0, b})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path, err := r.Finish(ctx)

	if err != nil || path == "" {
		t.Fatal("MP3 not exported", err)
	}

	decoded, err := exec.Command(encoder, "-v", "error", "-i", path, "-f", "s16le", "-ar", "24000", "-ac", "1", "pipe:1").Output()

	if err != nil || len(decoded) < 4800 {
		t.Fatal("MP3 invalid", err)
	}

	for _, frequency := range []float64{440, 880} {
		var re, im float64
		count := len(decoded) / 2

		for i := 0; i < count; i++ {
			sample := float64(int16(binary.LittleEndian.Uint16(decoded[i*2:])))
			phase := 2 * math.Pi * frequency * float64(i) / 24000
			re += sample * math.Cos(phase)
			im += sample * math.Sin(phase)
		}

		amplitude := 2 * math.Hypot(re, im) / float64(count)

		if amplitude < 1000 {
			t.Fatalf("recording missing %g Hz track: amplitude %g", frequency, amplitude)
		}
	}

	if r.Take() != path {
		t.Fatal("recording transfer failed")
	}

	r.Discard()

	if _, err := os.Stat(path); err != nil {
		t.Fatal("transferred output removed")
	}

	os.Remove(path)
}

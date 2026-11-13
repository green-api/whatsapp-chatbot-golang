package calls

import (
	"encoding/binary"
	"math"
	"testing"
)

func toneRMS(data []byte, start int) float64 {
	var power float64

	for i := start * 2; i+1 < len(data); i += 2 {
		s := float64(int16(binary.LittleEndian.Uint16(data[i:])))
		power += s * s
	}

	return math.Sqrt(power / float64(len(data)/2-start))
}

func TestOutputToneSoftensUpperVoiceWithoutChangingFrameSize(t *testing.T) {
	const rate = 24000
	const samples = 2400

	for _, frequency := range []float64{500, 7000} {
		raw := make([]byte, samples*2)

		for i := 0; i < samples; i++ {
			binary.LittleEndian.PutUint16(raw[i*2:], uint16(int16(10000*math.Sin(2*math.Pi*frequency*float64(i)/rate))))
		}

		filter := outputTone{mix: .2}

		var got []byte

		for i := 0; i < len(raw); i += PCMBytesPerFrame {
			got = append(got, filter.process(raw[i:i+PCMBytesPerFrame])...)
		}

		if len(got) != len(raw) {
			t.Fatalf("%g Hz: frame duration changed", frequency)
		}

		ratio := toneRMS(got, 480) / toneRMS(raw, 480)

		if frequency == 500 && (ratio < .97 || ratio > 1.01) {
			t.Fatalf("low voice changed too much: %.3f", ratio)
		}

		if frequency == 7000 && (ratio < .75 || ratio > .95) {
			t.Fatalf("upper voice not gently softened: %.3f", ratio)
		}
	}
}

func TestOutputToneDisabledKeepsPCM(t *testing.T) {
	data := []byte{0, 0, 255, 127}

	if got := (&outputTone{}).process(data); &got[0] != &data[0] {
		t.Fatal("disabled tone should pass through unchanged")
	}
}

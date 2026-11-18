package calls

import (
	"encoding/binary"
	"math"
	"testing"
)

func testTone(rate int, frequency float64, samples int) []int16 {
	pcm := make([]int16, samples)

	for i := range pcm {
		pcm[i] = int16(10000 * math.Sin(2*math.Pi*frequency*float64(i)/float64(rate)))
	}

	return pcm
}

func rmsPCM(data []byte, skip int) float64 {
	var power float64
	count := 0

	for i := skip * 2; i+1 < len(data); i += 2 {
		sample := float64(int16(binary.LittleEndian.Uint16(data[i:])))
		power += sample * sample
		count++
	}

	return math.Sqrt(power / float64(count))
}

func TestResamplerPreservesFrameDurationAndLowSpeechFrequencies(t *testing.T) {
	input := testTone(48000, 1000, 4800)
	converter := &pcmRateConverter{}
	var output []byte

	for i := 0; i < len(input); i += 960 {
		output = append(output, converter.downsample(input[i:i+960], 1)...)
	}

	if len(output) != len(input) {
		t.Fatalf("48 to 24 kHz duration changed: %d bytes", len(output))
	}

	if got := rmsPCM(output, 100); got < 6000 || got > 8000 {
		t.Fatalf("1 kHz speech band attenuated: RMS %.0f", got)
	}

	up := &pcmRateConverter{}
	for i := 0; i < len(output); i += PCMBytesPerFrame {
		frame := up.upsample(output[i : i+PCMBytesPerFrame])

		if len(frame) != 960 {
			t.Fatalf("20 ms SDK frame has %d samples", len(frame))
		}
	}
}

func TestResamplerRejectsFrequencyAboveVoiceNyquist(t *testing.T) {
	input := testTone(48000, 18000, 4800)
	converter := &pcmRateConverter{}
	var output []byte

	for i := 0; i < len(input); i += 960 {
		output = append(output, converter.downsample(input[i:i+960], 1)...)
	}

	if got := rmsPCM(output, 100); got > 500 {
		t.Fatalf("18 kHz aliased into voice band: RMS %.0f", got)
	}
}

package calls

import (
	"encoding/binary"
	"math"
)

// The SDK uses 48 kHz PCM while the internal voice and recorder path uses 24 kHz.
// A streaming low-pass filter suppresses aliasing and imaging at that boundary.
const resamplerTaps = 33

var resamplerKernel = makeResamplerKernel()

func makeResamplerKernel() [resamplerTaps]float64 {
	var kernel [resamplerTaps]float64
	const cutoff = .25
	const center = (resamplerTaps - 1) / 2
	var sum float64

	for i := range kernel {
		x := float64(i - center)
		value := 2 * cutoff

		if x != 0 {
			value = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}

		window := .54 - .46*math.Cos(2*math.Pi*float64(i)/float64(resamplerTaps-1))
		kernel[i] = value * window
		sum += kernel[i]
	}

	for i := range kernel {
		kernel[i] /= sum
	}

	return kernel
}

type pcmRateConverter struct {
	history [resamplerTaps]float64
	phase   int
}

func (r *pcmRateConverter) push(sample float64) float64 {
	copy(r.history[1:], r.history[:resamplerTaps-1])
	r.history[0] = sample
	var result float64

	for i, coefficient := range resamplerKernel {
		result += coefficient * r.history[i]
	}

	return result
}

func pcmSample(value float64) int16 {
	return int16(max(-32768, min(32767, math.Round(value))))
}

func (r *pcmRateConverter) upsample(frame []byte) []int16 {
	output := make([]int16, len(frame))

	for i := 0; i < len(frame)/2; i++ {
		sample := int16(binary.LittleEndian.Uint16(frame[i*2:]))
		output[i*2] = pcmSample(2 * r.push(float64(sample)))
		output[i*2+1] = pcmSample(2 * r.push(0))
	}

	return output
}

func (r *pcmRateConverter) downsample(frame []int16, channels int) []byte {
	count := len(frame) / channels
	output := make([]byte, 0, (count+1)/2*2)

	for i := 0; i < count; i++ {
		var sample float64

		for channel := 0; channel < channels; channel++ {
			sample += float64(frame[i*channels+channel])
		}

		sample /= float64(channels)
		filtered := r.push(sample)
		r.phase++

		if r.phase == 2 {
			output = binary.LittleEndian.AppendUint16(output, uint16(pcmSample(filtered)))
			r.phase = 0
		}
	}

	return output
}

func monoPCM(frame []int16, channels int) []byte {
	output := make([]byte, 0, len(frame)/channels*2)

	for i := 0; i < len(frame); i += channels {
		var sum int

		for channel := 0; channel < channels; channel++ {
			sum += int(frame[i+channel])
		}

		output = binary.LittleEndian.AppendUint16(output, uint16(int16(sum/channels)))
	}

	return output
}

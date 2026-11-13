package calls

import (
	"encoding/binary"
)

// outputTone gently reduces brightness without changing duration or pitch. The filter state persists between 20 ms
//  frames.
type outputTone struct {
	mix, smooth float64
}

func (t *outputTone) process(pcm []byte) []byte {
	if t.mix == 0 {
		return pcm
	}

    // 4.5 kHz one-pole low-pass at 24 kHz
	const alpha = 0.692137
	out := make([]byte, len(pcm))

	for i := 0; i+1 < len(pcm); i += 2 {
		sample := float64(int16(binary.LittleEndian.Uint16(pcm[i:])))
		t.smooth += alpha * (sample - t.smooth)
		value := pcmSample((1-t.mix)*sample + t.mix*t.smooth)
		binary.LittleEndian.PutUint16(out[i:], uint16(value))
	}

	return out
}

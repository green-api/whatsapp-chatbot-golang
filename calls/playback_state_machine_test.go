package calls

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestThreeMinuteResponseFitsAndKeepsOrder(t *testing.T) {
	p := NewPlaybackController(DefaultPlaybackOptions())

	for i := 0; i < 9000; i++ {
		frame := make([]byte, 960)

		for j := 0; j < 960; j += 2 {
			binary.LittleEndian.PutUint16(frame[j:], uint16(i))
		}

		r, err := p.Append("item", 0, frame)

		if err != nil || r != AppendResultAccepted {
			t.Fatalf("frame %d: %s %v", i, r, err)
		}
	}

	p.CompleteItem("item", 0)

	for i := 0; i < 9000; i++ {
		frame := p.ReadFrame()

		if int(binary.LittleEndian.Uint16(frame)) != i {
			t.Fatalf("frame %d out of order", i)
		}
	}

	if !bytes.Equal(p.ReadFrame(), make([]byte, 960)) || len(p.receivedSamples) != 0 {
		t.Fatal("finished playback did not release counters")
	}
}

func TestPartialFramePaddingKeepsExactHeardDuration(t *testing.T) {
	p := NewPlaybackController(DefaultPlaybackOptions())
	p.Append("item", 0, make([]byte, 500*2))
	p.FinishResponse()
	p.ReadFrame()
	cuts := p.Interrupt()

	if len(cuts) != 1 || cuts[0].PlayedMS != 20 {
		t.Fatal(cuts)
	}
}

func TestHardLimitDrainsAcceptedPrefixAndDropPolicyReusesCapacity(t *testing.T) {
	for _, policy := range []PlaybackOverflowPolicy{PlaybackOverflowPolicyDrainAndCancel, PlaybackOverflowPolicyDropChunks} {
		o := DefaultPlaybackOptions()
		o.HardLimitSeconds = .04
		o.SoftLimitSeconds = .02
		o.OverflowPolicy = policy
		p := NewPlaybackController(o)
		r, err := p.Append("item", 0, make([]byte, 960*3))

		if err != nil {
			t.Fatal(err)
		}

		if policy == PlaybackOverflowPolicyDrainAndCancel && r != AppendResultOverflow || policy == PlaybackOverflowPolicyDropChunks && r != AppendResultDropped {
			t.Fatal(r)
		}

		p.ReadFrame()
		p.ReadFrame()

		if policy == PlaybackOverflowPolicyDrainAndCancel {
			cuts := p.TakeDrainedCuts()

			if len(cuts) != 1 || cuts[0].PlayedMS != 40 {
				t.Fatal(cuts)
			}
		} else {
			r, _ = p.Append("item", 0, make([]byte, 960))

			if r != AppendResultAccepted {
				t.Fatal(r)
			}

			p.MarkFinishedResponse(map[AudioItemKey]bool{{"item", 0}: true})

			if len(p.TakeItemsToDelete()) != 1 {
				t.Fatal("missing audio item retained")
			}
		}
	}
}

func TestTrackReplacementAndStallDropUnheardAudio(t *testing.T) {
	p := NewPlaybackController(DefaultPlaybackOptions())
	p.Append("item", 0, make([]byte, 960*3))
	p.ReadFrame()
	cuts := p.ReplaceTrack()

	if len(cuts) != 1 || cuts[0].PlayedMS != 20 || p.BufferedBytes() != 0 {
		t.Fatal(cuts)
	}

	p.Append("new", 0, make([]byte, 960))
	p.mu.Lock()
	p.lastRead = time.Now().Add(-10 * time.Second)
	p.mu.Unlock()
	cuts = p.SuspendIfStalled()

	if len(cuts) != 1 || p.State() != PlaybackStateSuspended {
		t.Fatal(cuts)
	}
}

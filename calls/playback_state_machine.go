package calls

import (
	"errors"
	"sync"
	"time"
)

type PlaybackState string

type PlaybackEvent string

type AppendResult string

const (
	PlaybackStatePlaying       PlaybackState = "playing"
	PlaybackStateBacklog       PlaybackState = "backlog"
	PlaybackStateDraining      PlaybackState = "draining"
	PlaybackStateSuspended     PlaybackState = "suspended"
	PlaybackEventSoftLimit     PlaybackEvent = "soft_limit"
	PlaybackEventRecovered     PlaybackEvent = "recovered"
	PlaybackEventHardLimit     PlaybackEvent = "hard_limit"
	PlaybackEventDrained       PlaybackEvent = "drained"
	PlaybackEventStalled       PlaybackEvent = "stalled"
	PlaybackEventResumed       PlaybackEvent = "resumed"
	PlaybackEventInterrupted   PlaybackEvent = "interrupted"
	PlaybackEventTrackReplaced PlaybackEvent = "track_replaced"
	AppendResultAccepted       AppendResult  = "accepted"
	AppendResultOverflow       AppendResult  = "overflow"
	AppendResultDropped        AppendResult  = "dropped"
	AppendResultIgnored        AppendResult  = "ignored"
)

type PlaybackCut struct {
	ItemID                 string
	ContentIndex, PlayedMS int
}

type AudioItemKey struct {
	ItemID       string
	ContentIndex int
}

type playbackChunk struct {
	Key     AudioItemKey
	Data    []byte
	Samples int
}

type PlaybackStateMachine struct{ state PlaybackState }

func NewPlaybackStateMachine() *PlaybackStateMachine {
	return &PlaybackStateMachine{PlaybackStatePlaying}
}

func (f *PlaybackStateMachine) State() PlaybackState { return f.state }

func (f *PlaybackStateMachine) Apply(e PlaybackEvent) PlaybackState {
	switch e {
	case PlaybackEventHardLimit:
		f.state = PlaybackStateDraining
	case PlaybackEventStalled:
		f.state = PlaybackStateSuspended
	case PlaybackEventInterrupted, PlaybackEventTrackReplaced:
		f.state = PlaybackStatePlaying
	case PlaybackEventSoftLimit:
		if f.state == PlaybackStatePlaying {
			f.state = PlaybackStateBacklog
		}
	case PlaybackEventRecovered:
		if f.state == PlaybackStateBacklog {
			f.state = PlaybackStatePlaying
		}
	case PlaybackEventDrained:
		if f.state == PlaybackStateDraining {
			f.state = PlaybackStatePlaying
		}
	case PlaybackEventResumed:
		if f.state == PlaybackStateSuspended {
			f.state = PlaybackStatePlaying
		}
	}

	return f.state
}

// PlaybackController retains the Python policies; a mutex replaces its event-loop ownership.
type PlaybackController struct {
	mu                                                 sync.Mutex
	options                                            PlaybackOptions
	fsm                                                *PlaybackStateMachine
	chunks                                             []playbackChunk
	pending                                            []byte
	pendingItem                                        *AudioItemKey
	receivedSamples, playedSamples                     map[AudioItemKey]int
	completed, missing                                 map[AudioItemKey]bool
	deletions, queuedDeletions                         map[string]bool
	droppedSamples, bufferedBytes, peakBufferedSamples int
	lastRead                                           time.Time
	drainedCuts                                        []PlaybackCut
}

func NewPlaybackController(o PlaybackOptions) *PlaybackController {
	return &PlaybackController{options: o, fsm: NewPlaybackStateMachine(), receivedSamples: map[AudioItemKey]int{}, playedSamples: map[AudioItemKey]int{}, completed: map[AudioItemKey]bool{}, missing: map[AudioItemKey]bool{}, deletions: map[string]bool{}, queuedDeletions: map[string]bool{}, lastRead: time.Now()}
}

func (p *PlaybackController) State() PlaybackState {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.fsm.state
}

func (p *PlaybackController) DroppedSamples() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.droppedSamples
}

func (p *PlaybackController) BufferedBytes() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.bufferedBytes
}

func (p *PlaybackController) Append(item string, index int, data []byte) (AppendResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(data)%2 != 0 {
		return "", errors.New("audio must contain whole PCM samples")
	}

	key := AudioItemKey{item, index}

	if p.fsm.state == PlaybackStateDraining || p.fsm.state == PlaybackStateSuspended {
		return AppendResultIgnored, nil
	}

	p.receivedSamples[key] += len(data) / 2

	if p.pendingItem != nil && *p.pendingItem != key {
		p.flushPendingFrame()
	}

	capacity := max(1, int(p.options.HardLimitSeconds*PCMRate)/PCMSamplesPerFrame) * PCMBytesPerFrame
	n := min(len(data), max(0, capacity-p.bufferedBytes))

	if p.bufferedBytes == 0 && n > 0 {
		p.lastRead = time.Now()
	}

	p.pendingItem = &key
	p.bufferedBytes += n
	p.peakBufferedSamples = max(p.peakBufferedSamples, p.bufferedBytes/2)
	p.pending = append(p.pending, data[:n]...)
	p.splitFrames()

	if float64(p.bufferedBytes)/(PCMRate*2) >= p.options.SoftLimitSeconds {
		p.fsm.Apply(PlaybackEventSoftLimit)
	}

	if n == len(data) {
		return AppendResultAccepted, nil
	}

	p.flushPendingFrame()

	if p.options.OverflowPolicy == PlaybackOverflowPolicyDropChunks {
		p.missing[key] = true
		p.droppedSamples += (len(data) - n) / 2

		return AppendResultDropped, nil
	}

	p.fsm.Apply(PlaybackEventHardLimit)
	p.finishDrainIfEmpty()

	return AppendResultOverflow, nil
}

func (p *PlaybackController) splitFrames() {
	for len(p.pending) >= PCMBytesPerFrame {
		p.chunks = append(p.chunks, playbackChunk{*p.pendingItem, append([]byte(nil), p.pending[:PCMBytesPerFrame]...), PCMSamplesPerFrame})
		p.pending = p.pending[PCMBytesPerFrame:]
	}
}

func (p *PlaybackController) flushPendingFrame() {
	if len(p.pending) == 0 {
		p.pendingItem = nil

		return
	}

	data := make([]byte, PCMBytesPerFrame)
	copy(data, p.pending)
	p.chunks = append(p.chunks, playbackChunk{*p.pendingItem, data, len(p.pending) / 2})
	p.bufferedBytes += PCMBytesPerFrame - len(p.pending)
	p.pending = nil
	p.pendingItem = nil
}

func (p *PlaybackController) FinishResponse() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.flushPendingFrame()
}

func (p *PlaybackController) CompleteItem(item string, index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := AudioItemKey{item, index}

	if p.pendingItem != nil && *p.pendingItem == key {
		p.flushPendingFrame()
	}

	p.completed[key] = true
	p.pruneCompleted()
}

func (p *PlaybackController) ReadFrame() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastRead = time.Now()
	p.fsm.Apply(PlaybackEventResumed)

	if len(p.chunks) == 0 {
		return make([]byte, PCMBytesPerFrame)
	}

	c := p.chunks[0]
	p.chunks[0] = playbackChunk{}
	p.chunks = p.chunks[1:]
	p.playedSamples[c.Key] += c.Samples
	p.bufferedBytes -= len(c.Data)
	p.finishDrainIfEmpty()

	if p.fsm.state != PlaybackStateDraining {
		p.pruneCompleted()
	}

	if float64(p.bufferedBytes)/(PCMRate*2) < p.options.SoftLimitSeconds {
		p.fsm.Apply(PlaybackEventRecovered)
	}

	return c.Data
}

func (p *PlaybackController) finishDrainIfEmpty() {
	if p.fsm.state != PlaybackStateDraining || p.bufferedBytes != 0 {
		return
	}

	p.drainedCuts = p.prepareInterruption()

	if p.drainedCuts == nil {
		p.drainedCuts = []PlaybackCut{}
	}

	p.clear()
	p.fsm.Apply(PlaybackEventDrained)
}

func (p *PlaybackController) pruneCompleted() {
	if p.fsm.state == PlaybackStateDraining {
		return
	}

	buffered := map[AudioItemKey]bool{}

	for _, c := range p.chunks {
		buffered[c.Key] = true
	}

	if p.pendingItem != nil {
		buffered[*p.pendingItem] = true
	}

	for k := range p.completed {
		if buffered[k] {
			continue
		}

		if p.missing[k] {
			p.markItemForDeletion(k.ItemID)
			delete(p.missing, k)
		}

		delete(p.receivedSamples, k)
		delete(p.playedSamples, k)
		delete(p.completed, k)
	}
}

func (p *PlaybackController) TakeDrainedCuts() []PlaybackCut {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.drainedCuts
	p.drainedCuts = nil

	return r
}

func (p *PlaybackController) TakeItemsToDelete() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := []string{}

	for k := range p.deletions {
		r = append(r, k)
	}

	p.deletions = map[string]bool{}

	return r
}

func (p *PlaybackController) MarkFinishedResponse(keys map[AudioItemKey]bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k := range keys {
		if p.missing[k] {
			p.markItemForDeletion(k.ItemID)
		}
	}
}

func (p *PlaybackController) markItemForDeletion(k string) {
	if !p.queuedDeletions[k] {
		p.queuedDeletions[k] = true
		p.deletions[k] = true
	}
}

func (p *PlaybackController) prepareInterruption() []PlaybackCut {
	for k := range p.missing {
		p.markItemForDeletion(k.ItemID)
	}

	r := []PlaybackCut{}

	for k, n := range p.receivedSamples {
		played := p.playedSamples[k]

		if !p.missing[k] && n > played {
			r = append(r, PlaybackCut{k.ItemID, k.ContentIndex, played * 1000 / PCMRate})
		}
	}

	return r
}

func (p *PlaybackController) clear() {
	p.chunks = nil
	p.pending = nil
	p.pendingItem = nil
	p.receivedSamples = map[AudioItemKey]int{}
	p.playedSamples = map[AudioItemKey]int{}
	p.completed = map[AudioItemKey]bool{}
	p.missing = map[AudioItemKey]bool{}
	p.bufferedBytes = 0
}

func (p *PlaybackController) Interrupt() []PlaybackCut {
	p.mu.Lock()
	defer p.mu.Unlock()
	cuts := p.prepareInterruption()
	p.clear()
	p.drainedCuts = nil
	p.fsm.Apply(PlaybackEventInterrupted)

	return cuts
}

func (p *PlaybackController) ReplaceTrack() []PlaybackCut {
	p.mu.Lock()
	defer p.mu.Unlock()
	cuts := p.prepareInterruption()
	p.clear()
	p.drainedCuts = nil
	p.fsm.Apply(PlaybackEventTrackReplaced)
	p.lastRead = time.Now()

	return cuts
}

func (p *PlaybackController) SuspendIfStalled() []PlaybackCut {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bufferedBytes == 0 || time.Since(p.lastRead) < seconds(p.options.StallSeconds) {
		return nil
	}

	cuts := p.prepareInterruption()
	p.clear()
	p.drainedCuts = nil
	p.fsm.Apply(PlaybackEventStalled)

	return cuts
}

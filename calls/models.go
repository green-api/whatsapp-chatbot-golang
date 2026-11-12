// Package calls ports whatsapp_chatbot_python.calls. Sessions have one transition owner.
package calls

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type CallDirection string

const (
	CallDirectionIncoming CallDirection = "incoming"
	CallDirectionOutgoing CallDirection = "outgoing"
)

type InputOverflowPolicy string

const (
	InputOverflowPolicyDropOldest InputOverflowPolicy = "drop_oldest"
	InputOverflowPolicyFail       InputOverflowPolicy = "fail"
)

type PlaybackOverflowPolicy string

const (
	PlaybackOverflowPolicyDrainAndCancel PlaybackOverflowPolicy = "drain_and_cancel"
	PlaybackOverflowPolicyDropChunks     PlaybackOverflowPolicy = "drop_chunks"
)

type CallState string

const (
	CallStateTriggered   CallState = "triggered"
	CallStateQueued      CallState = "queued"
	CallStateDialing     CallState = "dialing"
	CallStateRinging     CallState = "ringing"
	CallStateConnecting  CallState = "connecting"
	CallStateInCall      CallState = "in_call"
	CallStateEnding      CallState = "ending"
	CallStateCancelled   CallState = "cancelled"
	CallStateRejected    CallState = "rejected"
	CallStateRingTimeout CallState = "ring_timeout"
	CallStateRemoteEnded CallState = "remote_ended"
	CallStateTalkTimeout CallState = "talk_timeout"
	CallStateFailed      CallState = "failed"
)

type CallEndReason string

const (
	CallEndReasonRemoteHangup    CallEndReason = "remote_hangup"
	CallEndReasonRemoteRejected  CallEndReason = "remote_rejected"
	CallEndReasonRingTimeout     CallEndReason = "ring_timeout"
	CallEndReasonTalkTimeout     CallEndReason = "talk_timeout"
	CallEndReasonError           CallEndReason = "error"
	CallEndReasonShutdown        CallEndReason = "shutdown"
	CallEndReasonUserCancelled   CallEndReason = "user_cancelled"
	CallEndReasonIncomingExpired CallEndReason = "incoming_expired"
	CallEndReasonCallerLeft      CallEndReason = "caller_left"
)

type CallEvent string

const (
	CallEventEnqueued          CallEvent = "enqueued"
	CallEventDequeued          CallEvent = "dequeued"
	CallEventCancelRequested   CallEvent = "cancel_requested"
	CallEventDialAccepted      CallEvent = "dial_accepted"
	CallEventIncomingAccepted  CallEvent = "incoming_accepted"
	CallEventRemoteAccepted    CallEvent = "remote_accepted"
	CallEventBridgeReady       CallEvent = "bridge_ready"
	CallEventRemoteIdle        CallEvent = "remote_idle"
	CallEventCallerLeft        CallEvent = "caller_left"
	CallEventIncomingExpired   CallEvent = "incoming_expired"
	CallEventRingTimerExpired  CallEvent = "ring_timer_expired"
	CallEventTalkTimerExpired  CallEvent = "talk_timer_expired"
	CallEventHangupConfirmed   CallEvent = "hangup_confirmed"
	CallEventInternalError     CallEvent = "internal_error"
	CallEventShutdownRequested CallEvent = "shutdown_requested"
)

type DeadlineKind string

const (
	DeadlineKindSetup  DeadlineKind = "setup"
	DeadlineKindOffer  DeadlineKind = "offer"
	DeadlineKindRing   DeadlineKind = "ring"
	DeadlineKindTalk   DeadlineKind = "talk"
	DeadlineKindBridge DeadlineKind = "bridge"
)

type Command string

const (
	CommandPrepare   Command = "prepare"
	CommandDial      Command = "dial"
	CommandAccept    Command = "accept"
	CommandReconcile Command = "reconcile"
	CommandBridge    Command = "bridge"
	CommandGreet     Command = "greet"
)

type AcceptResult string

const (
	AcceptResultCommandAcknowledged AcceptResult = "command_acknowledged"
	AcceptResultAlreadyActive       AcceptResult = "already_active"
)

type FinishResult string

const (
	FinishResultFinished     FinishResult = "finished"
	FinishResultAlreadyEnded FinishResult = "already_ended"
	FinishResultNotOwned     FinishResult = "not_owned"
	FinishResultUnknown      FinishResult = "unknown"
)

const (
	PCM16SampleBytes   = 2
	PCMRate            = 24000
	PCMSamplesPerFrame = 480
	PCMBytesPerFrame   = 960
	PCMFrameDuration   = 20 * time.Millisecond
)

type CallSession struct {
	SenderID, ChatID, Language, SessionID                string
	Direction                                            CallDirection
	State                                                CallState
	CallID                                               string
	IncomingDeadline                                     time.Time
	Prompt                                               *string
	EndReason                                            CallEndReason
	RemoteReason, ErrorCode                              string
	CreatedAt, QueuedAt, StartedAt, AnsweredAt, EndedAt  time.Time
	RemoteAccepted, BridgeReady, IncomingAcceptCompleted bool
}

func NewCallSession(senderID, chatID, language string) *CallSession {
	var id [16]byte
	_, _ = rand.Read(id[:])
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80

	return &CallSession{SenderID: senderID, ChatID: chatID, Language: language, SessionID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), Direction: CallDirectionOutgoing, State: CallStateTriggered, CreatedAt: time.Now().UTC()}
}

func NewIncomingCallSession(senderID, callID, language string, deadline time.Time) (*CallSession, error) {
	if senderID == "" || callID == "" || deadline.IsZero() {
		return nil, validationError("incoming call requires sender ID, call ID and deadline")
	}

	s := NewCallSession(senderID, senderID, language)
	s.Direction = CallDirectionIncoming
	s.CallID = callID
	s.IncomingDeadline = deadline

	return s, nil
}

func (s *CallSession) Finished() bool {
	switch s.State {
	case CallStateCancelled, CallStateRejected, CallStateRingTimeout, CallStateRemoteEnded, CallStateTalkTimeout, CallStateFailed:
		return true
	}

	return false
}

func (s *CallSession) Validate() error {
	if s.Prompt != nil && strings.TrimSpace(*s.Prompt) == "" {
		return validationError("call prompt must not be empty")
	}

	return nil
}

type StateTransition struct {
	Previous, Current CallState
	Event             CallEvent
	Applied           bool
}

func (t StateTransition) Changed() bool { return t.Previous != t.Current }

func (t StateTransition) Outcome() string {
	if t.Applied {
		return "applied"
	}

	return "ignored"
}

type TransitionDetails struct{ RemoteReason, ErrorCode *string }

type TransitionCallback func(*CallSession, CallEvent, ...TransitionDetails) StateTransition

type CallExecutionResult struct {
	mu            sync.Mutex
	recordingPath string
}

func NewCallExecutionResult(path string) *CallExecutionResult {
	return &CallExecutionResult{recordingPath: path}
}

func (r *CallExecutionResult) RecordingPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recordingPath
}

func (r *CallExecutionResult) TakeRecording() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.recordingPath
	r.recordingPath = ""

	return p
}

func (r *CallExecutionResult) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recordingPath == "" {
		return nil
	}

	err := os.Remove(r.recordingPath)

	if err == nil || errors.Is(err, os.ErrNotExist) {
		r.recordingPath = ""

		return nil
	}

	return err
}

type CallExecutor interface {
	Execute(context.Context, *CallSession, TransitionCallback) (*CallExecutionResult, error)
	RequestStop()
}

type IncomingCallExecutor interface {
	CallExecutor
	RejectIncoming(context.Context, string, string) (bool, error)
	NotifyIncomingEnded(string) bool
}

type PcmChunk struct {
	Position int64
	Data     []byte
}

type RecordingOptions struct {
	Enabled                                          bool
	BitrateBPS, MaxPendingChunks, ExportBlockSamples int
	WriterPollSeconds                                float64
	Directory, EncoderPath                           string
}

func DefaultRecordingOptions() RecordingOptions {
	return RecordingOptions{Enabled: true, BitrateBPS: 96000, MaxPendingChunks: 256, WriterPollSeconds: .1, ExportBlockSamples: 24000, EncoderPath: "ffmpeg"}
}

type PlaybackOptions struct {
	SoftLimitSeconds, HardLimitSeconds, StallSeconds, WatchdogSeconds float64
	OverflowPolicy                                                    PlaybackOverflowPolicy
}

func DefaultPlaybackOptions() PlaybackOptions {
	return PlaybackOptions{30, 180, 6, 1, PlaybackOverflowPolicyDrainAndCancel}
}

type InputAudioOptions struct {
	BufferSeconds, SendSeconds, EnqueueSeconds float64
	OverflowPolicy                             InputOverflowPolicy
}

func DefaultInputAudioOptions() InputAudioOptions {
	return InputAudioOptions{3, 2, .25, InputOverflowPolicyDropOldest}
}

type VadOptions struct {
	Threshold                          float64
	PrefixPaddingMS, SilenceDurationMS int
	NoiseReduction                     string
}

func DefaultVadOptions() VadOptions { return VadOptions{.7, 400, 800, "near_field"} }

type AudioLoss struct {
	Direction      string
	DroppedSamples int64
	ObservedAt     time.Time
}

func (a AudioLoss) DroppedSeconds() float64 { return float64(a.DroppedSamples) / PCMRate }

type CallOptions struct {
	RingSeconds, TalkSeconds, SetupSeconds, CommandSeconds, ShutdownSeconds float64
	Recording                                                               RecordingOptions
	Playback                                                                PlaybackOptions
	InputAudio                                                              InputAudioOptions
	Vad                                                                     VadOptions
	OnAudioLoss                                                             func(*CallSession, AudioLoss)
}

func DefaultCallOptions() CallOptions {
	return CallOptions{RingSeconds: 30, TalkSeconds: 240, SetupSeconds: 30, CommandSeconds: 10, ShutdownSeconds: 10, Recording: DefaultRecordingOptions(), Playback: DefaultPlaybackOptions(), InputAudio: DefaultInputAudioOptions(), Vad: DefaultVadOptions()}
}

func positiveValues(v ...float64) error {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 {
			return validationError("options must be finite and positive")
		}
	}

	return nil
}

func (o CallOptions) Validate() error {
	if err := positiveValues(o.RingSeconds, o.TalkSeconds, o.SetupSeconds, o.CommandSeconds, o.ShutdownSeconds, o.Playback.SoftLimitSeconds, o.Playback.HardLimitSeconds, o.Playback.StallSeconds, o.Playback.WatchdogSeconds, o.InputAudio.BufferSeconds, o.InputAudio.SendSeconds, o.InputAudio.EnqueueSeconds, float64(o.Recording.BitrateBPS), float64(o.Recording.MaxPendingChunks), o.Recording.WriterPollSeconds, float64(o.Recording.ExportBlockSamples)); err != nil {
		return err
	}

	if o.Playback.SoftLimitSeconds >= o.Playback.HardLimitSeconds {
		return validationError("playback soft limit must be below hard limit")
	}

	if o.Playback.OverflowPolicy != PlaybackOverflowPolicyDrainAndCancel && o.Playback.OverflowPolicy != PlaybackOverflowPolicyDropChunks {
		return validationError("unknown playback overflow policy")
	}

	if o.InputAudio.OverflowPolicy != InputOverflowPolicyDropOldest && o.InputAudio.OverflowPolicy != InputOverflowPolicyFail {
		return validationError("unknown input overflow policy")
	}

	return o.Vad.Validate()
}

func (o VadOptions) Validate() error {
	if math.IsNaN(o.Threshold) || math.IsInf(o.Threshold, 0) || o.Threshold < 0 || o.Threshold > 1 || o.PrefixPaddingMS < 0 || o.SilenceDurationMS <= 0 {
		return validationError("invalid VAD options")
	}

	if o.NoiseReduction != "" && o.NoiseReduction != "near_field" && o.NoiseReduction != "far_field" {
		return validationError("unknown noise reduction")
	}

	return nil
}

type ProviderConfig struct {
	APIURL, IDInstance, APITokenInstance, OpenAIAPIKey, RealtimeModel, RealtimeVoice string
	Instructions                                                                     *string
	RealtimeURL                                                                      string
}

func (c ProviderConfig) String() string {
	return fmt.Sprintf("ProviderConfig{APIURL:%q IDInstance:%q Model:%q Voice:%q}", c.APIURL, c.IDInstance, c.RealtimeModel, c.RealtimeVoice)
}

func (c ProviderConfig) GoString() string { return c.String() }

type ProviderError struct{ Provider, Code, Detail string }

func (e *ProviderError) Error() string { return e.Provider + " [" + e.Code + "]" }

type CallCleanupError struct {
	Stage string
	Cause error
}

func (e *CallCleanupError) Error() string { return "call cleanup failed at " + e.Stage }

func (e *CallCleanupError) Unwrap() error { return e.Cause }

// CallValidationError preserves Python's ValueError code while retaining a Go cause.
type CallValidationError struct{ Cause error }

func (e *CallValidationError) Error() string { return e.Cause.Error() }

func (e *CallValidationError) Unwrap() error { return e.Cause }

func validationError(message string) error { return &CallValidationError{Cause: errors.New(message)} }

func errorCode(err error) string {
	var cleanup *CallCleanupError

	if errors.As(err, &cleanup) {
		return "cleanup_" + cleanup.Stage + "_" + errorCode(cleanup.Cause)
	}

	var p *ProviderError

	if errors.As(err, &p) {
		return p.Code
	}

	var validation *CallValidationError

	if errors.As(err, &validation) {
		return "ValueError"
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "TimeoutError"
	}

	return "RuntimeError"
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

type RuntimeEvent struct {
	Kind           string
	ObservedAt     time.Time
	Command        Command
	Result         AcceptResult
	Error          error
	CallID, Reason string
	Deadline       DeadlineKind
	Loss           AudioLoss
}

type EventSink func(RuntimeEvent) bool

func runtimeEvent(kind string) RuntimeEvent { return RuntimeEvent{Kind: kind, ObservedAt: time.Now()} }

type Deadlines struct {
	due   map[DeadlineKind]time.Time
	order []DeadlineKind
}

func NewDeadlines() *Deadlines { return &Deadlines{due: make(map[DeadlineKind]time.Time)} }

func (d *Deadlines) Arm(k DeadlineKind, at time.Time) {
	if _, ok := d.due[k]; !ok {
		d.due[k] = at
		d.order = append(d.order, k)
	}
}

func (d *Deadlines) Disarm(ks ...DeadlineKind) {
	for _, k := range ks {
		delete(d.due, k)

		for i, x := range d.order {
			if x == k {
				d.order = append(d.order[:i], d.order[i+1:]...)
				break
			}
		}
	}
}

func (d *Deadlines) next() (DeadlineKind, time.Time) {
	var kind DeadlineKind
	var at time.Time

	for _, k := range d.order {
		v := d.due[k]

		if at.IsZero() || v.Before(at) {
			kind, at = k, v
		}
	}

	return kind, at
}

func (d *Deadlines) ExpireBefore(at time.Time) *RuntimeEvent {
	k, t := d.next()

	if t.IsZero() || t.After(at) {
		return nil
	}

	d.Disarm(k)

	return &RuntimeEvent{Kind: "deadline_expired", ObservedAt: t, Deadline: k}
}

// PcmFormat is the fixed internal Python audio format. SDK conversion stays at the boundary.
type PcmFormat struct {
	Rate, FrameMS, Channels int
	Layout, AVFormat        string
}

func DefaultPcmFormat() PcmFormat {
	return PcmFormat{Rate: PCMRate, FrameMS: 20, Channels: 1, Layout: "mono", AVFormat: "s16"}
}

func (f PcmFormat) Validate() error {
	if f.Rate != PCMRate || f.FrameMS <= 0 || f.Channels != 1 || f.Rate*f.FrameMS%1000 != 0 {
		return validationError("only 24 kHz mono PCM with integral positive frames is supported")
	}

	return nil
}

func (f PcmFormat) SamplesPerFrame() int { return f.Rate * f.FrameMS / 1000 }

func (f PcmFormat) BytesPerFrame() int { return f.SamplesPerFrame() * PCM16SampleBytes }

func (f PcmFormat) FrameSeconds() float64 { return float64(f.FrameMS) / 1000 }

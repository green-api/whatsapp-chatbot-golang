package calls

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Events is an unbounded FIFO, matching asyncio.Queue. Deadline ordering uses observation time.
type Events struct {
	mu      sync.Mutex
	queue   []RuntimeEvent
	pending *RuntimeEvent
	closed  bool
	wake    chan struct{}
}

func NewEvents() *Events { return &Events{wake: make(chan struct{}, 1)} }

func (e *Events) Emit(event RuntimeEvent) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return false
	}

	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now()
	}

	e.queue = append(e.queue, event)

	select {
	case e.wake <- struct{}{}:
	default:
	}

	return true
}

func (e *Events) Receive(ctx context.Context, d *Deadlines) (RuntimeEvent, error) {
	for {
		e.mu.Lock()
		var event *RuntimeEvent

		if e.pending != nil {
			event = e.pending
			e.pending = nil
		} else if len(e.queue) > 0 {
			x := e.queue[0]
			e.queue[0] = RuntimeEvent{}
			e.queue = e.queue[1:]
			event = &x
		}

		closed := e.closed
		e.mu.Unlock()

		if event != nil {
			if expired := d.ExpireBefore(event.ObservedAt); expired != nil {
				e.mu.Lock()
				e.pending = event
				e.mu.Unlock()

				return *expired, nil
			}

			return *event, nil
		}

		if closed {
			return RuntimeEvent{}, errors.New("events closed")
		}

		_, at := d.next()
		var timer *time.Timer
		var tick <-chan time.Time

		if !at.IsZero() {
			timer = time.NewTimer(max(0, time.Until(at)))
			tick = timer.C
		}

		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}

			return RuntimeEvent{}, ctx.Err()
		case <-e.wake:
			if timer != nil {
				timer.Stop()
			}
		case <-tick:
			// Re-enter the FIFO before synthesizing expiry: a previously observed event wins.
			e.mu.Lock()
			queued := len(e.queue) > 0 || e.pending != nil
			e.mu.Unlock()

			if !queued {
				if expired := d.ExpireBefore(time.Now()); expired != nil {
					return *expired, nil
				}
			}
		}
	}
}

func (e *Events) Close() {
	e.mu.Lock()
	e.closed = true
	e.queue = nil
	e.pending = nil
	e.mu.Unlock()

	select {
	case e.wake <- struct{}{}:
	default:
	}
}

type operation struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type Operations struct {
	mu       sync.Mutex
	tasks    map[Command]*operation
	stopping bool
	emit     EventSink
}

func NewOperations(emit EventSink) *Operations {
	return &Operations{tasks: map[Command]*operation{}, emit: emit}
}

func (o *Operations) Start(command Command, fn func(context.Context) (AcceptResult, error), timeout float64) {
	o.mu.Lock()

	if o.stopping || o.tasks[command] != nil {
		o.mu.Unlock()

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), seconds(timeout))
	op := &operation{cancel, make(chan struct{})}
	o.tasks[command] = op
	o.mu.Unlock()

	type outcome struct {
		result AcceptResult
		err    error
	}

	outcomes := make(chan outcome, 1)

	go func() { defer close(op.done); result, err := fn(ctx); outcomes <- outcome{result, err} }()

	go func() {
		defer cancel()
		var result AcceptResult
		var err error

		select {
		case out := <-outcomes:
			result, err = out.result, out.err
		case <-ctx.Done():
			err = ctx.Err()
		}

		o.mu.Lock()
		stopping := o.stopping
		o.mu.Unlock()

		if !stopping {
			event := RuntimeEvent{Kind: "command_completed", Command: command, Result: result}

			if err != nil {
				event.Kind = "command_failed"
				event.Error = err
			}

			o.emit(event)
		}
	}()
}

func (o *Operations) BeginStop() {
	o.mu.Lock()
	o.stopping = true

	for _, op := range o.tasks {
		op.cancel()
	}

	o.mu.Unlock()
}

func (o *Operations) Close() error {
	o.BeginStop()
	o.mu.Lock()
	tasks := make([]*operation, 0, len(o.tasks))

	for _, op := range o.tasks {
		tasks = append(tasks, op)
	}

	o.mu.Unlock()

	for _, op := range tasks {
		<-op.done
	}

	return nil
}

func (o *Operations) PendingTaskCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := 0

	for _, op := range o.tasks {
		select {
		case <-op.done:
		default:
			n++
		}
	}

	return n
}

type CallProvider interface {
	Open(context.Context, *AudioBridge) error
	Dial(context.Context) error
	Accept(context.Context) (AcceptResult, error)
	ReconcileAccept(context.Context, error) error
	StartAudio(context.Context) error
	Finish(context.Context) (FinishResult, error)
	RejectOffer(context.Context) (bool, error)
	Close() error
}

type VoiceConversation interface {
	AudioSource
	AudioSink
	Start(context.Context) error
	PromptOnce(context.Context, *string) error
	Close() error
}

type Dependencies struct {
	NewAPI   func(ProviderConfig, *CallSession, EventSink) CallProvider
	NewVoice func(ProviderConfig, PlaybackOptions, EventSink, string, float64, VadOptions) VoiceConversation
}

func defaultDependencies() Dependencies {
	return Dependencies{NewAPI: func(c ProviderConfig, s *CallSession, e EventSink) CallProvider { return NewGreenApiCall(c, s, e) }, NewVoice: func(c ProviderConfig, p PlaybackOptions, e EventSink, l string, shutdown float64, v VadOptions) VoiceConversation {
		return NewRealtimeConversation(c, p, e, l, shutdown, v)
	}}
}

type CleanupResult struct {
	IsRemoteFinishUnknown, IsLocalCloseUnknown bool
	UnfinishedTaskCount                        int
	IsRecordingExportRunning                   bool
}

type CallResources struct {
	options       CallOptions
	session       *CallSession
	Operations    *Operations
	Recorder      *CallRecorder
	Voice         VoiceConversation
	Audio         *AudioBridge
	API           CallProvider
	FinishResult  FinishResult
	cleanupMu     sync.Mutex
	cleanupErrors []*CallCleanupError
	pending       atomic.Int32
	closing       atomic.Bool
}

func newCallResources(c ProviderConfig, o CallOptions, s *CallSession, emit EventSink, deps Dependencies) *CallResources {
	r := &CallResources{options: o, session: s, Operations: NewOperations(emit), Recorder: NewCallRecorder(o.Recording)}
	r.Voice = deps.NewVoice(c, o.Playback, emit, s.Language, o.ShutdownSeconds, o.Vad)
	r.Audio = NewAudioBridge(r.Voice, r.Voice, r.Recorder, emit, o.InputAudio, o.OutputSoftening)
	r.API = deps.NewAPI(c, s, emit)

	return r
}

func (r *CallResources) Prepare(ctx context.Context) error {
	if err := r.Voice.Start(ctx); err != nil {
		return err
	}

	if r.closing.Load() {
		return errors.New("call resources are closing")
	}

	return r.API.Open(ctx, r.Audio)
}

func (r *CallResources) recordCloseError(stage string, err error) {
	r.cleanupMu.Lock()
	r.cleanupErrors = append(r.cleanupErrors, &CallCleanupError{stage, err})
	r.cleanupMu.Unlock()
	slog.Error("Call cleanup failed", "stage", stage, "error_type", fmt.Sprintf("%T", err))
}

func (r *CallResources) closeStep(stage string, fn func() error) {
	done := make(chan error, 1)
	r.pending.Add(1)

	go func() { defer r.pending.Add(-1); done <- fn() }()

	timeout := r.options.ShutdownSeconds

	if stage == "greenapi_close" {
		timeout *= 2
	}

	timer := time.NewTimer(seconds(timeout))
	defer timer.Stop()

	select {
	case err := <-done:
		if err != nil {
			r.recordCloseError(stage, err)
		}
	case <-timer.C:
		r.recordCloseError(stage, context.DeadlineExceeded)

		go func() {
			if err := <-done; err != nil {
				slog.Error("Late call cleanup failed", "stage", stage, "error_type", fmt.Sprintf("%T", err))
			}
		}()
	}
}

func (r *CallResources) Close() error {
	r.closing.Store(true)
	r.Operations.BeginStop()
	r.closeStep("operations_close", r.Operations.Close)
	// Operations that ignore cancellation retain their owners until they finish.
	// Other stages still run bounded waits; acquisition guards close late resources.
	r.closeStep("remote_finish", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), seconds(r.options.ShutdownSeconds))
		defer cancel()
		result, err := r.API.Finish(ctx)
		r.cleanupMu.Lock()
		r.FinishResult = result
		r.cleanupMu.Unlock()

		return err
	})
	r.closeStep("greenapi_close", r.API.Close)
	r.closeStep("audio_close", r.Audio.Close)
	r.closeStep("voice_close", r.Voice.Close)
	keepRecording := r.session.EndReason == CallEndReasonRemoteHangup || r.session.EndReason == CallEndReasonTalkTimeout
	r.closeStep("recording_close", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), seconds(r.options.ShutdownSeconds))
		defer cancel()

		if keepRecording {
			_, err := r.Recorder.Finish(ctx)

			return err
		}

		return r.Recorder.Close(ctx)
	})
	r.cleanupMu.Lock()
	defer r.cleanupMu.Unlock()

	if len(r.cleanupErrors) > 0 && !r.session.Finished() && r.FinishResult == "" {
		return r.cleanupErrors[0]
	}

	return nil
}

func (r *CallResources) CleanupResult() CleanupResult {
	r.cleanupMu.Lock()
	defer r.cleanupMu.Unlock()
	out := CleanupResult{UnfinishedTaskCount: int(r.pending.Load()) + r.Operations.PendingTaskCount(), IsRecordingExportRunning: r.Recorder.ExportRunning()}
	out.IsRemoteFinishUnknown = r.FinishResult == FinishResultUnknown

	for _, e := range r.cleanupErrors {
		if e.Stage == "remote_finish" {
			out.IsRemoteFinishUnknown = true
		} else {
			out.IsLocalCloseUnknown = true
		}
	}

	out.IsLocalCloseUnknown = out.IsLocalCloseUnknown || out.UnfinishedTaskCount > 0

	return out
}

func (r *CallResources) TakeResult(keep bool) *CallExecutionResult {
	if !keep {
		r.Recorder.Discard()

		return NewCallExecutionResult("")
	}

	return NewCallExecutionResult(r.Recorder.TakeReadyOrDiscard())
}

type CallRunner struct {
	Session         *CallSession
	transition      TransitionCallback
	config          ProviderConfig
	options         CallOptions
	deps            Dependencies
	events          *Events
	deadlines       *Deadlines
	resources       *CallResources
	incomingCallID  string
	incoming        bool
	admissionMu     sync.Mutex
	acceptingEvents bool
}

func newCallRunner(s *CallSession, t TransitionCallback, c ProviderConfig, o CallOptions, deps Dependencies) *CallRunner {
	return &CallRunner{Session: s, transition: t, config: c, options: o, deps: deps, events: NewEvents(), deadlines: NewDeadlines(), incoming: s.Direction == CallDirectionIncoming, incomingCallID: s.CallID, acceptingEvents: true}
}

func (r *CallRunner) RequestStop() { r.events.Emit(runtimeEvent("stop_requested")) }

func (r *CallRunner) NotifyIncomingEnded(id string) bool {
	r.admissionMu.Lock()
	defer r.admissionMu.Unlock()

	if !r.incoming || id != r.incomingCallID || !r.acceptingEvents {
		return false
	}

	return r.events.Emit(RuntimeEvent{Kind: "caller_left", CallID: id})
}

func (r *CallRunner) apply(e CallEvent, d ...TransitionDetails) StateTransition {
	// Close admission before a terminal transition. Callbacks may re-enter the
	// service, so they must run outside admissionMu.
	if rule, ok := callRules[transitionKey{r.Session.State, e}]; ok && (rule.Direction == "" || rule.Direction == r.Session.Direction) {
		change := rule.Change

		if rule.Resolver != nil {
			change = rule.Resolver(r.Session)
		}

		if (&CallSession{State: change.Target}).Finished() || change.Target == CallStateEnding {
			r.stopAcceptingEvents()
		}
	}

	transition := r.transition(r.Session, e, d...)

	if r.Session.Finished() || r.Session.State == CallStateEnding {
		r.stopAcceptingEvents()
	}

	return transition
}

func (r *CallRunner) stopAcceptingEvents() {
	r.admissionMu.Lock()
	r.acceptingEvents = false
	r.admissionMu.Unlock()
}

func (r *CallRunner) offerExpired() bool {
	return r.incoming && !r.Session.IncomingDeadline.After(time.Now())
}

func (r *CallRunner) Run(ctx context.Context) (result *CallExecutionResult, err error) {
	defer r.events.Close()
	defer r.stopAcceptingEvents()
	s := r.Session

	defer func() {
		if err != nil {
			event := CallEventInternalError

			if ctx.Err() != nil {
				event = CallEventShutdownRequested
			}

			code := errorCode(err)
			r.apply(event, TransitionDetails{ErrorCode: &code})
		}
	}()

	if err = s.Validate(); err != nil {
		return nil, err
	}

	expected := CallStateDialing

	if r.incoming {
		expected = CallStateRinging
	}

	if s.State != expected {
		return nil, validationError("dequeue session before executing a call")
	}

	if r.incoming && (s.CallID == "" || s.IncomingDeadline.IsZero()) {
		return nil, validationError("incoming call requires ID and deadline")
	}

	if r.offerExpired() {
		r.apply(CallEventIncomingExpired)
		api := r.deps.NewAPI(r.config, s, r.events.Emit)
		c, cancel := context.WithTimeout(context.Background(), seconds(r.options.CommandSeconds))
		_, _ = api.RejectOffer(c)
		cancel()
		_ = api.Close()

		return NewCallExecutionResult(""), nil
	}

	r.resources = newCallResources(r.config, r.options, s, r.events.Emit, r.deps)
	defer r.resources.Recorder.Discard()
	r.deadlines.Arm(DeadlineKindSetup, time.Now().Add(seconds(r.options.SetupSeconds)))

	if !s.IncomingDeadline.IsZero() {
		r.deadlines.Arm(DeadlineKindOffer, s.IncomingDeadline)
	}

	r.start(CommandPrepare, func(c context.Context) (AcceptResult, error) { return "", r.resources.Prepare(c) }, r.options.SetupSeconds)

	for !s.Finished() && s.State != CallStateEnding {
		var event RuntimeEvent
		event, err = r.events.Receive(ctx, r.deadlines)

		if err != nil {
			break
		}

		if err = r.dispatch(event); err != nil {
			break
		}
	}

	r.stopAcceptingEvents()

	if err != nil {
		event := CallEventInternalError

		if ctx.Err() != nil {
			event = CallEventShutdownRequested
		}

		code := errorCode(err)
		r.apply(event, TransitionDetails{ErrorCode: &code})
	}

	cleanupErr := r.resources.Close()

	if err == nil {
		err = cleanupErr
	}

	if err != nil {
		return nil, err
	}

	if s.State == CallStateEnding {
		r.resources.cleanupMu.Lock()
		finish := r.resources.FinishResult
		r.resources.cleanupMu.Unlock()

		if finish == FinishResultFinished || finish == FinishResultAlreadyEnded {
			r.apply(CallEventHangupConfirmed)
		} else {
			code := "finish_" + string(finish)

			if finish == "" {
				code = "finish_missing_result"
			}

			r.apply(CallEventInternalError, TransitionDetails{ErrorCode: &code})
		}
	}

	return r.resources.TakeResult(s.State == CallStateRemoteEnded || s.State == CallStateTalkTimeout), nil
}

func (r *CallRunner) start(cmd Command, fn func(context.Context) (AcceptResult, error), timeout ...float64) {
	limit := r.options.CommandSeconds

	if len(timeout) > 0 {
		limit = timeout[0]
	}

	r.resources.Operations.Start(cmd, fn, limit)
}

func (r *CallRunner) dispatch(e RuntimeEvent) error {
	switch e.Kind {
	case "command_completed":
		return r.commandCompleted(e.Command, e.ObservedAt, e.Result)
	case "command_failed":
		return r.commandFailed(e.Command, e.Error)
	case "remote_answered":
		r.answered(e.CallID, e.ObservedAt)
	case "remote_ended":
		r.apply(CallEventRemoteIdle, TransitionDetails{RemoteReason: &e.Reason})
	case "caller_left":
		r.callerLeft(e.CallID)
	case "stop_requested":
		r.apply(CallEventShutdownRequested)
	case "deadline_expired":
		return r.deadline(e.Deadline)
	case "media_failed":
		return e.Error
	case "audio_loss":
		if r.options.OnAudioLoss != nil {
			func() {
				defer func() {
					if recover() != nil {
						slog.Warn("Audio loss callback panicked")
					}
				}()

				r.options.OnAudioLoss(r.Session, e.Loss)
			}()
		}
	}

	return nil
}

func (r *CallRunner) commandCompleted(cmd Command, at time.Time, result AcceptResult) error {
	switch cmd {
	case CommandPrepare:
		r.deadlines.Disarm(DeadlineKindSetup)

		if r.incoming {
			if r.offerExpired() {
				r.apply(CallEventIncomingExpired)

				return nil
			}

			r.start(CommandAccept, r.resources.API.Accept)
		} else {
			r.start(CommandDial, func(ctx context.Context) (AcceptResult, error) { return "", r.resources.API.Dial(ctx) })
		}
	case CommandDial:
		r.apply(CallEventDialAccepted)

		if r.Session.State == CallStateRinging {
			r.deadlines.Arm(DeadlineKindRing, at.Add(seconds(r.options.RingSeconds)))
		}

		r.beginBridge()
	case CommandAccept:
		if result == AcceptResultCommandAcknowledged {
			r.apply(CallEventIncomingAccepted)
		}

		r.beginBridge()
	case CommandBridge:
		r.deadlines.Disarm(DeadlineKindBridge)
		r.apply(CallEventBridgeReady)
		r.activateConversation(at)
	}

	return nil
}

func (r *CallRunner) commandFailed(cmd Command, err error) error {
	if cmd == CommandAccept || cmd == CommandReconcile {
		if errorCode(err) == "offer_gone" {
			event := CallEventCallerLeft

			if r.offerExpired() {
				event = CallEventIncomingExpired
			}

			r.apply(event)

			return nil
		}

		if r.Session.RemoteAccepted {
			return nil
		}

		if cmd == CommandAccept {
			r.start(CommandReconcile, func(ctx context.Context) (AcceptResult, error) { return "", r.resources.API.ReconcileAccept(ctx, err) })

			return nil
		}
	}

	return err
}

func (r *CallRunner) answered(id string, at time.Time) {
	if r.incoming && id != r.Session.CallID || r.Session.RemoteAccepted {
		return
	}

	r.apply(CallEventRemoteAccepted)
	r.deadlines.Disarm(DeadlineKindOffer, DeadlineKindRing)
	r.beginBridge()
	r.activateConversation(at)
}

func (r *CallRunner) beginBridge() {
	if r.Session.BridgeReady {
		return
	}

	r.deadlines.Arm(DeadlineKindBridge, time.Now().Add(seconds(r.options.SetupSeconds)))
	r.start(CommandBridge, func(ctx context.Context) (AcceptResult, error) { return "", r.resources.API.StartAudio(ctx) }, r.options.SetupSeconds)
}

func (r *CallRunner) activateConversation(at time.Time) {
	if r.Session.State != CallStateInCall {
		return
	}

	r.deadlines.Disarm(DeadlineKindBridge)
	r.deadlines.Arm(DeadlineKindTalk, at.Add(seconds(r.options.TalkSeconds)))
	r.resources.Audio.ActivateRecording()
	r.start(CommandGreet, func(ctx context.Context) (AcceptResult, error) {
		return "", r.resources.Voice.PromptOnce(ctx, r.Session.Prompt)
	})
}

func (r *CallRunner) callerLeft(id string) {
	if !r.incoming || id != r.Session.CallID {
		return
	}

	event := CallEventCallerLeft

	if r.Session.RemoteAccepted {
		event = CallEventRemoteIdle
	}

	r.apply(event)
}

func (r *CallRunner) deadline(k DeadlineKind) error {
	event := map[DeadlineKind]CallEvent{DeadlineKindOffer: CallEventIncomingExpired, DeadlineKindRing: CallEventRingTimerExpired, DeadlineKindTalk: CallEventTalkTimerExpired}[k]

	if event == "" {
		return context.DeadlineExceeded
	}

	r.apply(event)

	return nil
}

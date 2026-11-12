package calls

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Scenario and providers mirror tests/calls/fakes.py. Mocks stay next to the modules.
type scenario struct {
	mu                  sync.Mutex
	actions             []string
	gates               map[string]chan struct{}
	fail                string
	answer, remoteEnd   bool
	acceptBeforeFailure bool
	emit                EventSink
	voice               *fakeVoice
	api                 *fakeProvider
}

func newScenario() *scenario {
	return &scenario{gates: map[string]chan struct{}{}, answer: true, remoteEnd: true}
}

func (s *scenario) action(ctx context.Context, name string) error {
	s.mu.Lock()
	s.actions = append(s.actions, name)
	gate := s.gates[name]
	fail := s.fail == name
	s.mu.Unlock()

	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if fail {
		return errors.New("fake " + name + " failed")
	}

	return nil
}

func (s *scenario) has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.actions {
		if a == name {
			return true
		}
	}

	return false
}

func (s *scenario) wait(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)

	for !s.has(name) {
		if time.Now().After(deadline) {
			t.Fatalf("missing action %s", name)
		}

		time.Sleep(time.Millisecond)
	}
}

type fakeVoice struct {
	s    *scenario
	emit EventSink
}

func (v *fakeVoice) Start(ctx context.Context) error { return v.s.action(ctx, "voice_start") }

func (v *fakeVoice) PromptOnce(ctx context.Context, p *string) error {
	err := v.s.action(ctx, "greet")

	if v.s.remoteEnd && err == nil {
		v.emit(RuntimeEvent{Kind: "remote_ended", Reason: "normal"})
	}

	return err
}

func (v *fakeVoice) ReadFrame() []byte { return make([]byte, PCMBytesPerFrame) }

func (v *fakeVoice) ReplaceTrack() {}

func (v *fakeVoice) WriteAudio(ctx context.Context, b []byte) error { return nil }

func (v *fakeVoice) Close() error { return v.s.action(context.Background(), "voice_close") }

type fakeProvider struct {
	s    *scenario
	emit EventSink
	id   string
}

func (a *fakeProvider) Open(ctx context.Context, b *AudioBridge) error {
	return a.s.action(ctx, "open")
}

func (a *fakeProvider) Dial(ctx context.Context) error {
	err := a.s.action(ctx, "dial")

	if err == nil && a.s.answer {
		a.emit(RuntimeEvent{Kind: "remote_answered", CallID: a.id})
	}

	return err
}

func (a *fakeProvider) Accept(ctx context.Context) (AcceptResult, error) {
	if a.s.acceptBeforeFailure {
		a.emit(RuntimeEvent{Kind: "remote_answered", CallID: a.id})
	}

	err := a.s.action(ctx, "accept")

	if err == nil && a.s.answer {
		a.emit(RuntimeEvent{Kind: "remote_answered", CallID: a.id})
	}

	return AcceptResultCommandAcknowledged, err
}

func (a *fakeProvider) ReconcileAccept(ctx context.Context, err error) error {
	a.s.action(ctx, "reconcile")

	return err
}

func (a *fakeProvider) StartAudio(ctx context.Context) error { return a.s.action(ctx, "bridge") }

func (a *fakeProvider) Finish(ctx context.Context) (FinishResult, error) {
	err := a.s.action(ctx, "finish")

	return FinishResultFinished, err
}

func (a *fakeProvider) RejectOffer(ctx context.Context) (bool, error) {
	return true, a.s.action(ctx, "reject")
}

func (a *fakeProvider) Close() error { return a.s.action(context.Background(), "rtc_close") }

func (s *scenario) dependencies() Dependencies {
	return Dependencies{NewVoice: func(c ProviderConfig, p PlaybackOptions, e EventSink, l string, f float64, v VadOptions) VoiceConversation {
		s.voice = &fakeVoice{s, e}

		return s.voice
	}, NewAPI: func(c ProviderConfig, session *CallSession, e EventSink) CallProvider {
		s.api = &fakeProvider{s, e, session.CallID}
		s.emit = e

		return s.api
	}}
}

func testConfig() ProviderConfig {
	return ProviderConfig{IDInstance: "1", APITokenInstance: "token", OpenAIAPIKey: "key", RealtimeModel: "test", RealtimeVoice: "test"}
}

func testOptions() CallOptions {
	o := DefaultCallOptions()
	o.Recording.Enabled = false
	o.RingSeconds = .04
	o.TalkSeconds = .04
	o.SetupSeconds = .1
	o.CommandSeconds = .1
	o.ShutdownSeconds = .03

	return o
}

func testSession(incoming bool) *CallSession {
	s := NewCallSession("sender@c.us", "sender@c.us", "ru")

	if incoming {
		s.Direction = CallDirectionIncoming
		s.CallID = "call-1"
		s.IncomingDeadline = time.Now().Add(time.Second)
	}

	fsm := NewCallStateMachine()
	fsm.Apply(s, CallEventEnqueued)
	fsm.Apply(s, CallEventDequeued)

	return s
}

func TestCallServicePythonScenarios(t *testing.T) {
	for _, incoming := range []bool{false, true} {
		for _, which := range []string{"normal", "voice_start", "open", "bridge", "greet", "ring_timeout", "talk_timeout", "accept_failure_after_answer"} {
			t.Run(string(map[bool]CallDirection{true: CallDirectionIncoming, false: CallDirectionOutgoing}[incoming])+"/"+which, func(t *testing.T) {
				if !incoming && which == "accept_failure_after_answer" {
					t.Skip("incoming only")
				}

				sc := newScenario()
				sc.fail = which

				if which == "ring_timeout" {
					sc.answer = false
				}

				if which == "talk_timeout" {
					sc.remoteEnd = false
				}

				if which == "accept_failure_after_answer" {
					sc.fail = "accept"
					sc.acceptBeforeFailure = true
				}

				service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())

				if err != nil {
					t.Fatal(err)
				}

				s := testSession(incoming)

				if which == "ring_timeout" && incoming {
					s.IncomingDeadline = time.Now().Add(40 * time.Millisecond)
				}

				result, err := service.Execute(context.Background(), s, NewCallStateMachine().Apply)
				failure := which == "voice_start" || which == "open" || which == "bridge" || which == "greet"

				if failure {
					if err == nil || s.State != CallStateFailed {
						t.Fatalf("state=%s err=%v", s.State, err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					result.Close()
					expected := CallStateRemoteEnded

					if which == "ring_timeout" {
						expected = CallStateRingTimeout

						if incoming {
							expected = CallStateRejected
						}
					}

					if which == "talk_timeout" {
						expected = CallStateTalkTimeout
					}

					if s.State != expected {
						t.Fatalf("want %s got %s", expected, s.State)
					}
				}

				if !sc.has("voice_close") || !sc.has("rtc_close") {
					t.Fatal("resources not closed")
				}

				if (which == "voice_start" || which == "open") && sc.has("dial") {
					t.Fatal("dialed after setup failure")
				}
			})
		}
	}
}

func TestCancellationDrainsCleanupAtEachActiveStage(t *testing.T) {
	for _, stage := range []string{"voice_start", "open", "dial", "accept", "bridge"} {
		t.Run(stage, func(t *testing.T) {
			sc := newScenario()
			sc.remoteEnd = false
			sc.gates[stage] = make(chan struct{})
			service, _ := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())
			s := testSession(stage == "accept")
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)

			go func() { _, err := service.Execute(ctx, s, NewCallStateMachine().Apply); done <- err }()

			sc.wait(t, stage)
			cancel()

			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("cleanup blocked")
			}

			if s.State != CallStateCancelled || !sc.has("rtc_close") || !sc.has("voice_close") {
				t.Fatalf("cleanup/state mismatch %s", s.State)
			}
		})
	}
}

func TestExpiredIncomingOfferStartsNoVoice(t *testing.T) {
	sc := newScenario()
	service, _ := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())
	s := testSession(true)
	s.IncomingDeadline = time.Now().Add(-time.Second)
	result, err := service.Execute(context.Background(), s, NewCallStateMachine().Apply)

	if err != nil {
		t.Fatal(err)
	}

	result.Close()

	if s.EndReason != CallEndReasonIncomingExpired || sc.has("voice_start") || !sc.has("reject") {
		t.Fatal("expired offer semantics")
	}
}

func TestCallerLeftDuringPendingAccept(t *testing.T) {
	sc := newScenario()
	sc.gates["accept"] = make(chan struct{})
	service, _ := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())
	s := testSession(true)
	done := make(chan error, 1)

	go func() { _, err := service.Execute(context.Background(), s, NewCallStateMachine().Apply); done <- err }()

	sc.wait(t, "accept")

	if service.NotifyIncomingEnded("wrong") {
		t.Fatal("wrong ID accepted")
	}

	if !service.NotifyIncomingEnded(s.CallID) {
		t.Fatal("matching ID ignored")
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if s.EndReason != CallEndReasonCallerLeft {
		t.Fatal(s.EndReason)
	}
}

func TestCleanupFailureAllowsSubsequentCall(t *testing.T) {
	sc := newScenario()
	sc.fail = "rtc_close"
	service, _ := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())

	for i := 0; i < 2; i++ {
		s := testSession(false)
		r, err := service.Execute(context.Background(), s, NewCallStateMachine().Apply)

		if err != nil {
			t.Fatal(err)
		}

		r.Close()

		if s.State != CallStateRemoteEnded || !service.Healthy() {
			t.Fatal("cleanup disabled admission")
		}
	}
}

func TestStoppedServiceDoesNotStartNewCall(t *testing.T) {
	sc := newScenario()
	service, _ := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())
	service.RequestStop()
	s := testSession(false)
	_, err := service.Execute(context.Background(), s, NewCallStateMachine().Apply)

	if err != nil || s.State != CallStateCancelled || sc.has("voice_start") || service.Healthy() {
		t.Fatal("stopped admission")
	}
}

func TestObservationBeforeDeadlineWinsAndBoundaryIsLate(t *testing.T) {
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		d := NewDeadlines()
		at := time.Now().Add(-time.Second)
		d.Arm(DeadlineKindRing, at)
		events := NewEvents()
		events.Emit(RuntimeEvent{Kind: "remote_answered", ObservedAt: at.Add(offset)})
		e, err := events.Receive(context.Background(), d)

		if err != nil {
			t.Fatal(err)
		}

		if offset < 0 && e.Kind != "remote_answered" || offset >= 0 && e.Kind != "deadline_expired" {
			t.Fatalf("offset %v kind %s", offset, e.Kind)
		}

		if offset >= 0 {
			e, _ = events.Receive(context.Background(), d)

			if e.Kind != "remote_answered" {
				t.Fatal("pending event lost")
			}
		}
	}
}

func TestNamedOperationRunsOnceAndReportsTimeout(t *testing.T) {
	events := NewEvents()
	ops := NewOperations(events.Emit)
	var calls int

	ops.Start(CommandDial, func(ctx context.Context) (AcceptResult, error) { calls++; <-ctx.Done(); return "", ctx.Err() }, .01)

	ops.Start(CommandDial, func(context.Context) (AcceptResult, error) { t.Error("duplicate operation"); return "", nil }, 1)
	e, err := events.Receive(context.Background(), NewDeadlines())
	ops.Close()

	if err != nil || e.Kind != "command_failed" || !errors.Is(e.Error, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("event=%+v err=%v", e, err)
	}
}

func TestIncomingEndNotificationRejectedDuringCleanup(t *testing.T) {
	sc := newScenario()
	gate := make(chan struct{})
	sc.gates["rtc_close"] = gate
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())

	if err != nil {
		t.Fatal(err)
	}

	session := testSession(true)
	done := make(chan error, 1)

	go func() {
		_, err := service.Execute(context.Background(), session, NewCallStateMachine().Apply)
		done <- err
	}()

	sc.wait(t, "rtc_close")

	if service.NotifyIncomingEnded(session.CallID) {
		t.Fatal("terminal incoming call accepted a notification during cleanup")
	}

	close(gate)

	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestInvalidStartKeepsPythonValueErrorCode(t *testing.T) {
	sc := newScenario()
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())

	if err != nil {
		t.Fatal(err)
	}

	session := NewCallSession("sender", "chat", "en")
	_, err = service.Execute(context.Background(), session, NewCallStateMachine().Apply)

	if err == nil || session.State != CallStateFailed || session.ErrorCode != "ValueError" {
		t.Fatalf("invalid start lost Python validation code: state=%s code=%s err=%v", session.State, session.ErrorCode, err)
	}
}

package calls_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/green-api/whatsapp-chatbot-golang/calls"
)

type cleanupProbe struct {
	mu      sync.Mutex
	actions []string
	gate    chan struct{}
}

func (p *cleanupProbe) add(action string) {
	p.mu.Lock()
	p.actions = append(p.actions, action)
	p.mu.Unlock()
}

func (p *cleanupProbe) has(action string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, got := range p.actions {
		if got == action {
			return true
		}
	}

	return false
}

type cleanupVoice struct{ probe *cleanupProbe }

func (v *cleanupVoice) Start(context.Context) error               { return nil }
func (v *cleanupVoice) PromptOnce(context.Context, *string) error { return nil }
func (v *cleanupVoice) ReadFrame() []byte                         { return make([]byte, 960) }
func (v *cleanupVoice) ReplaceTrack()                             {}
func (v *cleanupVoice) WriteAudio(context.Context, []byte) error  { return nil }
func (v *cleanupVoice) Close() error {
	v.probe.add("voice_close")

	return nil
}

type cleanupProvider struct{ probe *cleanupProbe }

func (p *cleanupProvider) Open(context.Context, *calls.AudioBridge) error { return nil }
func (p *cleanupProvider) Dial(context.Context) error                     { return nil }
func (p *cleanupProvider) Accept(context.Context) (calls.AcceptResult, error) {
	return calls.AcceptResultCommandAcknowledged, nil
}
func (p *cleanupProvider) ReconcileAccept(context.Context, error) error { return nil }
func (p *cleanupProvider) StartAudio(context.Context) error             { return nil }
func (p *cleanupProvider) Finish(context.Context) (calls.FinishResult, error) {
	p.probe.add("finish")

	return calls.FinishResultFinished, nil
}
func (p *cleanupProvider) RejectOffer(context.Context) (bool, error) { return true, nil }
func (p *cleanupProvider) Close() error {
	p.probe.add("provider_close")

	if p.probe.gate != nil {
		<-p.probe.gate
	}

	return nil
}

func newCleanupService(probe *cleanupProbe) *calls.WhatsAppCallService {
	options := calls.DefaultCallOptions()
	options.Recording.Enabled = false
	options.RingSeconds = .02
	options.TalkSeconds = .02
	options.ShutdownSeconds = .01
	deps := calls.Dependencies{
		NewAPI: func(calls.ProviderConfig, *calls.CallSession, calls.EventSink) calls.CallProvider {
			return &cleanupProvider{probe}
		},
		NewVoice: func(calls.ProviderConfig, calls.PlaybackOptions, calls.EventSink, string, float64, calls.VadOptions) calls.VoiceConversation {
			return &cleanupVoice{probe}
		},
	}
	service, err := calls.NewWhatsAppCallServiceWithDependencies(calls.ProviderConfig{
		IDInstance: "instance", APITokenInstance: "token", OpenAIAPIKey: "key",
	}, options, deps)

	if err != nil {
		panic(err)
	}

	return service
}

func TestCallCleanupWaitsForBlockedProviderAndClosesVoice(t *testing.T) {
	probe := &cleanupProbe{gate: make(chan struct{})}
	service := newCleanupService(probe)
	session := calls.NewCallSession("sender", "chat", "en")
	fsm := calls.NewCallStateMachine()
	fsm.Apply(session, calls.CallEventEnqueued)
	fsm.Apply(session, calls.CallEventDequeued)
	done := make(chan error, 1)

	go func() {
		_, err := service.Execute(context.Background(), session, fsm.Apply)
		done <- err
	}()

	deadline := time.After(time.Second)

	for !probe.has("provider_close") {
		select {
		case <-deadline:
			t.Fatal("provider cleanup did not start")
		case <-time.After(time.Millisecond):
		}
	}

	select {
	case <-done:
		t.Fatal("Execute returned before blocked provider close was released")
	default:
	}

	close(probe.gate)

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if !probe.has("voice_close") {
		t.Fatal("voice cleanup was skipped")
	}
}

func TestCallCleanupFailureDoesNotDisableLaterCalls(t *testing.T) {
	probe := &cleanupProbe{}
	service := newCleanupService(probe)

	for i := 0; i < 2; i++ {
		session := calls.NewCallSession("sender", "chat", "en")
		fsm := calls.NewCallStateMachine()
		fsm.Apply(session, calls.CallEventEnqueued)
		fsm.Apply(session, calls.CallEventDequeued)

		if _, err := service.Execute(context.Background(), session, fsm.Apply); err != nil {
			t.Fatal(err)
		}
	}

	if !service.Healthy() {
		t.Fatal("service admission was disabled after cleanup")
	}
}

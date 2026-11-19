package calls

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// WhatsAppCallService is single-instance. RequestStop permanently closes admission.
// Session fields belong to the Execute goroutine; external readers must synchronize.
type WhatsAppCallService struct {
	config  ProviderConfig
	options CallOptions
	deps    Dependencies
	gate    chan struct{}
	mu      sync.Mutex
	runner  *callRunner
	stopped bool
}

// NewWhatsAppCallService uses GREEN-API for calls and OpenAI Realtime for voice.
func NewWhatsAppCallService(config ProviderConfig, options ...CallOptions) (*WhatsAppCallService, error) {
	return NewWhatsAppCallServiceWithVoice(config, nil, options...)
}

// NewWhatsAppCallServiceWithVoice uses GREEN-API for calls and creates a voice
// conversation with factory for each call. A nil factory selects OpenAI Realtime
// and requires config.OpenAIAPIKey. An explicit factory takes precedence.
func NewWhatsAppCallServiceWithVoice(config ProviderConfig, factory VoiceFactory, options ...CallOptions) (*WhatsAppCallService, error) {
	o := DefaultCallOptions()

	if len(options) > 1 {
		return nil, validationError("expected at most one CallOptions")
	}

	if len(options) > 0 {
		o = options[0]
	}

	if factory == nil && config.OpenAIAPIKey == "" {
		return nil, validationError("provide OpenAIAPIKey or a custom voice provider")
	}

	deps := defaultDependencies()

	if factory != nil {
		deps.NewVoice = func(_ ProviderConfig, playback PlaybackOptions, emit EventSink, language string, shutdown float64, vad VadOptions) VoiceConversation {
			return factory(VoiceParams{Language: language, Playback: playback, VAD: vad, ShutdownSeconds: shutdown, Emit: voiceEventSink(emit)})
		}
	}

	return NewWhatsAppCallServiceWithDependencies(config, o, deps)
}

func voiceEventSink(emit EventSink) EventSink {
	return func(event RuntimeEvent) bool {
		switch event.Kind {
		case "media_failed":
			if event.Error != nil {
				return emit(RuntimeEvent{Kind: "media_failed", Error: event.Error})
			}
		case "audio_loss":
			return emit(RuntimeEvent{Kind: "audio_loss", Loss: event.Loss})
		}

		return false
	}
}

// NewWhatsAppCallServiceWithDependencies accepts explicit transport and voice factories.
// The voice factory is responsible for its provider's credentials.
func NewWhatsAppCallServiceWithDependencies(config ProviderConfig, o CallOptions, deps Dependencies) (*WhatsAppCallService, error) {
	if config.IDInstance == "" || config.APITokenInstance == "" {
		return nil, validationError("GREEN-API IDInstance and APITokenInstance are required")
	}

	if err := o.Validate(); err != nil {
		return nil, err
	}

	if deps.NewAPI == nil || deps.NewVoice == nil {
		return nil, validationError("provider factories required")
	}

	return &WhatsAppCallService{config: config, options: o, deps: deps, gate: make(chan struct{}, 1)}, nil
}

func (s *WhatsAppCallService) Healthy() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.stopped }

func (s *WhatsAppCallService) RequestStop() {
	s.mu.Lock()
	s.stopped = true
	r := s.runner
	s.mu.Unlock()

	if r != nil {
		r.RequestStop()
	}
}

// NotifyIncomingEnded reports queue acceptance, not completed processing.
// Notifications queued during a transition are ignored if the call has ended.
func (s *WhatsAppCallService) NotifyIncomingEnded(id string) bool {
	s.mu.Lock()
	r := s.runner
	s.mu.Unlock()

	return r != nil && r.NotifyIncomingEnded(id)
}

func (s *WhatsAppCallService) RejectIncoming(ctx context.Context, id, sender string) (bool, error) {
	session, err := NewIncomingCallSession(sender, id, "", time.Now().Add(time.Hour))

	if err != nil {
		return false, err
	}

	api := s.deps.NewAPI(s.config, session, func(RuntimeEvent) bool { return true })
	defer api.Close()
	ctx, cancel := context.WithTimeout(ctx, seconds(s.options.CommandSeconds))
	defer cancel()

	return api.RejectOffer(ctx)
}

func (s *WhatsAppCallService) Execute(ctx context.Context, session *CallSession, transition TransitionCallback) (*CallExecutionResult, error) {
	if session == nil || transition == nil {
		return nil, validationError("session and transition callback required")
	}

	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	defer func() { <-s.gate }()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	stopped := s.stopped

	if session.Finished() || stopped {
		s.mu.Unlock()
		already := session.Finished()

		if !already {
			transition(session, CallEventShutdownRequested)
		}

		if stopped && !already && session.Direction == CallDirectionIncoming && session.CallID != "" {
			_, _ = s.RejectIncoming(ctx, session.CallID, session.SenderID)
		}

		return NewCallExecutionResult(""), nil
	}

	runner := newCallRunner(session, transition, s.config, s.options, s.deps)
	s.runner = runner
	s.mu.Unlock()

	defer func() {
		if runner.resources != nil {
			out := runner.resources.cleanupResult()

			if out.IsRemoteFinishUnknown || out.IsLocalCloseUnknown || out.IsRecordingExportRunning {
				slog.Error("Call cleanup not fully finished; later call attempts remain enabled", "session", session.SessionID, "cleanup", out)
			}
		}

		s.mu.Lock()
		s.runner = nil
		s.mu.Unlock()
	}()

	return runner.Run(ctx)
}

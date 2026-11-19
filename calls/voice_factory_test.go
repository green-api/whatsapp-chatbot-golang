package calls

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCustomVoiceFactoryCreatesAndClosesOneConversationPerCall(t *testing.T) {
	options := testOptions()
	options.SetupSeconds = 1
	var params []VoiceParams
	var voices []*audioTestVoice
	service, err := NewWhatsAppCallServiceWithVoice(ProviderConfig{IDInstance: "1", APITokenInstance: "token"}, func(p VoiceParams) VoiceConversation {
		params = append(params, p)
		voice := &audioTestVoice{fakeVoice: fakeVoice{s: newScenario(), emit: p.Emit}, input: make(chan int, 1)}
		voices = append(voices, voice)
		return voice
	}, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) != 0 {
		t.Fatal("factory called during service construction")
	}
	service.deps.NewAPI = func(_ ProviderConfig, s *CallSession, emit EventSink) CallProvider {
		voice := voices[len(voices)-1]
		provider := &audioTestProvider{fakeProvider: fakeProvider{s: voice.s, emit: emit, id: s.CallID}, voice: voice}
		voice.onGreeting = provider.endCall
		return provider
	}

	for i, language := range []string{"ru", "en"} {
		session := testSession(i == 1)
		session.Language = language
		result, err := service.Execute(context.Background(), session, NewCallStateMachine().Apply)
		if err != nil {
			t.Fatal(err)
		}
		result.Close()
		if len(voices) != i+1 || session.State != CallStateRemoteEnded || session.RemoteReason != "normal" {
			t.Fatalf("got %d conversations, call state %s, remote reason %q", len(voices), session.State, session.RemoteReason)
		}
		p := params[i]
		if p.Language != language || p.Playback != options.Playback || p.VAD != options.Vad || p.ShutdownSeconds != options.ShutdownSeconds || p.Emit == nil {
			t.Fatalf("voice parameters do not match call and options: %+v", p)
		}
		if !voices[i].s.has("voice_start") || !voices[i].s.has("voice_close") || !voices[i].outputRead {
			t.Fatal("custom conversation did not complete its audio lifecycle")
		}
	}
}

type audioTestVoice struct {
	fakeVoice
	input      chan int
	outputRead bool
	onGreeting func()
}

func (v *audioTestVoice) PromptOnce(ctx context.Context, _ *string) error {
	if err := v.s.action(ctx, "greet"); err != nil {
		return err
	}
	if v.emit(RuntimeEvent{Kind: "remote_ended"}) {
		return errors.New("voice was allowed to signal a remote hangup")
	}
	v.onGreeting()
	return nil
}

func (v *audioTestVoice) ReadFrame() []byte {
	v.outputRead = true
	return v.fakeVoice.ReadFrame()
}

func (v *audioTestVoice) WriteAudio(ctx context.Context, pcm []byte) error {
	select {
	case v.input <- len(pcm):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type audioTestProvider struct {
	fakeProvider
	voice *audioTestVoice
}

func (p *audioTestProvider) endCall() {
	p.emit(RuntimeEvent{Kind: "remote_ended", Reason: "normal"})
}

func (p *audioTestProvider) Open(ctx context.Context, bridge *AudioBridge) error {
	audio, err := bridge.Create(ctx)
	if err != nil {
		return err
	}
	if _, err := bridge.track.Recv(ctx); err != nil {
		return err
	}
	if err := audio.OnRemoteTrack(ctx, &oneRemote{}); err != nil {
		return err
	}
	select {
	case size := <-p.voice.input:
		if size != PCMBytesPerFrame {
			return errors.New("unexpected caller PCM frame size")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCustomVoiceFactoryFailureAndCancellation(t *testing.T) {
	for _, stage := range []string{"start failure", "cancel start"} {
		t.Run(stage, func(t *testing.T) {
			scenario := newScenario()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if stage == "cancel start" {
				scenario.gates["voice_start"] = make(chan struct{})
			} else {
				scenario.fail = "voice_start"
			}
			options := testOptions()
			options.SetupSeconds = 1
			service, err := NewWhatsAppCallServiceWithVoice(ProviderConfig{IDInstance: "1", APITokenInstance: "token"}, func(p VoiceParams) VoiceConversation {
				return &fakeVoice{s: scenario, emit: p.Emit}
			}, options)
			if err != nil {
				t.Fatal(err)
			}
			session := testSession(false)
			done := make(chan error, 1)
			go func() {
				_, err := service.Execute(ctx, session, NewCallStateMachine().Apply)
				done <- err
			}()
			if stage == "cancel start" {
				scenario.wait(t, "voice_start")
				cancel()
			}
			err = <-done
			if stage == "cancel start" {
				if !errors.Is(err, context.Canceled) || session.State != CallStateCancelled {
					t.Fatalf("got state %s and error %v", session.State, err)
				}
			} else if err == nil || session.State != CallStateFailed {
				t.Fatalf("got state %s and error %v", session.State, err)
			}
			if !scenario.has("voice_close") {
				t.Fatal("custom conversation was not closed")
			}
		})
	}
}

func TestCustomVoiceFactoryRejectsNilConversation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		voice VoiceConversation
	}{
		{name: "nil interface"},
		{name: "typed nil", voice: (*fakeVoice)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{IDInstance: "1", APITokenInstance: "token"}
			service, err := NewWhatsAppCallServiceWithVoice(config, func(VoiceParams) VoiceConversation { return tc.voice }, testOptions())
			if err != nil {
				t.Fatal(err)
			}
			session := testSession(false)
			result, err := service.Execute(context.Background(), session, NewCallStateMachine().Apply)
			var validation *CallValidationError
			if !errors.As(err, &validation) || session.State != CallStateFailed || result != nil {
				t.Fatalf("wanted validation failure, got state %s, result %v, error %v", session.State, result, err)
			}
		})
	}
}

func TestVoiceEventSinkOnlyForwardsVoiceEvents(t *testing.T) {
	failure := errors.New("media failure")
	loss := AudioLoss{Direction: "caller", DroppedSamples: 960}
	for _, tc := range []struct {
		name    string
		event   RuntimeEvent
		want    RuntimeEvent
		forward bool
	}{
		{name: "media failure", event: RuntimeEvent{Kind: "media_failed", Error: failure, CallID: "other"}, want: RuntimeEvent{Kind: "media_failed", Error: failure}, forward: true},
		{name: "audio loss", event: RuntimeEvent{Kind: "audio_loss", Loss: loss}, want: RuntimeEvent{Kind: "audio_loss", Loss: loss}, forward: true},
		{name: "missing error", event: RuntimeEvent{Kind: "media_failed"}},
		{name: "remote hangup", event: RuntimeEvent{Kind: "remote_ended"}},
		{name: "command completed", event: RuntimeEvent{Kind: "command_completed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forwarded := false
			emit := voiceEventSink(func(event RuntimeEvent) bool {
				forwarded = true
				if event != tc.want {
					t.Errorf("forwarded event %+v, want %+v", event, tc.want)
				}
				return true
			})
			if accepted := emit(tc.event); accepted != tc.forward || forwarded != tc.forward {
				t.Fatalf("accepted = %t, forwarded = %t, want both %t", accepted, forwarded, tc.forward)
			}
		})
	}
	emit := voiceEventSink(func(RuntimeEvent) bool { return false })
	if emit(RuntimeEvent{Kind: "media_failed", Error: failure}) {
		t.Fatal("reported acceptance after the queue rejected a valid event")
	}
}

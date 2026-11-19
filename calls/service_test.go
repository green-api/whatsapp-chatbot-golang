package calls

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCallServiceConstructorValidatesContract(t *testing.T) {
	deps := newScenario().dependencies()
	options := testOptions()

	for _, tc := range []struct {
		name   string
		config ProviderConfig
		deps   Dependencies
		valid  bool
	}{
		{name: "valid", config: testConfig(), deps: deps, valid: true},
		{name: "missing instance", config: ProviderConfig{APITokenInstance: "token", OpenAIAPIKey: "key"}, deps: deps},
		{name: "missing token", config: ProviderConfig{IDInstance: "instance", OpenAIAPIKey: "key"}, deps: deps},
		{name: "custom voice without key", config: ProviderConfig{IDInstance: "instance", APITokenInstance: "token"}, deps: deps, valid: true},
		{name: "missing transport factory", config: testConfig(), deps: Dependencies{NewVoice: deps.NewVoice}},
		{name: "missing factory", config: testConfig(), deps: Dependencies{NewAPI: deps.NewAPI}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewWhatsAppCallServiceWithDependencies(tc.config, options, tc.deps)

			if tc.valid && err != nil {
				t.Fatal(err)
			}

			if !tc.valid && err == nil {
				t.Fatal("invalid service configuration was accepted")
			}
		})
	}

	if _, err := NewWhatsAppCallService(testConfig(), options, options); err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("unexpected variadic option validation: %v", err)
	}
}

func TestCallServiceSelectsVoice(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     string
		custom  bool
		wantErr string
	}{
		{name: "OpenAI", key: "key"},
		{name: "missing voice", wantErr: "provide OpenAIAPIKey or a custom voice provider"},
		{name: "custom without key", custom: true},
		{name: "custom takes precedence", key: "key", custom: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := testConfig()
			config.OpenAIAPIKey = tc.key
			var service *WhatsAppCallService
			var err error
			custom := &fakeVoice{s: newScenario()}
			if tc.custom {
				service, err = NewWhatsAppCallServiceWithVoice(config, func(VoiceParams) VoiceConversation { return custom })
			} else {
				service, err = NewWhatsAppCallService(config)
			}

			if tc.wantErr != "" {
				var validation *CallValidationError
				if !errors.As(err, &validation) || err.Error() != tc.wantErr {
					t.Fatalf("wanted validation error %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			resources := newCallResources(config, service.options, testSession(false), func(RuntimeEvent) bool { return true }, service.deps)
			t.Cleanup(func() { _ = resources.Close() })
			if _, ok := resources.API.(*GreenApiCall); !ok {
				t.Fatalf("transport is %T, want GREEN-API", resources.API)
			}
			if tc.custom && resources.Voice != custom {
				t.Fatalf("voice is %T, want the custom conversation", resources.Voice)
			}
			if _, ok := resources.Voice.(*RealtimeConversation); !tc.custom && !ok {
				t.Fatalf("voice is %T, want OpenAI Realtime", resources.Voice)
			}
		})
	}
}

func TestCallServiceWithVoiceRequiresGreenAPICredentials(t *testing.T) {
	for _, config := range []ProviderConfig{{IDInstance: "instance"}, {APITokenInstance: "token"}} {
		_, err := NewWhatsAppCallServiceWithVoice(config, func(VoiceParams) VoiceConversation {
			t.Fatal("factory called during construction")
			return nil
		})
		var validation *CallValidationError
		if !errors.As(err, &validation) || !strings.Contains(err.Error(), "GREEN-API") {
			t.Fatalf("wanted GREEN-API validation error, got %v", err)
		}
	}
}

func TestCallServiceStoppedAdmissionAndIncomingEndNotification(t *testing.T) {
	scenario := newScenario()
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), scenario.dependencies())

	if err != nil {
		t.Fatal(err)
	}

	if !service.Healthy() || service.NotifyIncomingEnded("call") {
		t.Fatal("new service should be healthy with no active incoming call")
	}

	service.RequestStop()

	if service.Healthy() {
		t.Fatal("RequestStop did not close service admission")
	}

	session := testSession(false)
	result, err := service.Execute(context.Background(), session, NewCallStateMachine().Apply)

	if err != nil || result == nil || session.State != CallStateCancelled || session.EndReason != CallEndReasonShutdown {
		t.Fatalf("stopped service did not reject new work cleanly: result=%v session=%+v err=%v", result, session, err)
	}
}

func TestCallServiceExecuteRejectsMissingArguments(t *testing.T) {
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), newScenario().dependencies())

	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Execute(context.Background(), nil, NewCallStateMachine().Apply); err == nil {
		t.Fatal("nil session accepted")
	}

	if _, err := service.Execute(context.Background(), testSession(false), nil); err == nil {
		t.Fatal("nil transition callback accepted")
	}
}

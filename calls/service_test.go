package calls

import (
	"context"
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
		{name: "missing voice key", config: ProviderConfig{IDInstance: "instance", APITokenInstance: "token"}, deps: deps},
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

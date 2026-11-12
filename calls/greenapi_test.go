package calls

import (
	"context"
	"errors"
	sdk "github.com/green-api/whatsapp-api-client-golang-v2/voip"
	"sync"
	"testing"
	"time"
)

type mockCallAPI struct {
	mu      sync.Mutex
	state   sdk.CallState
	actions []string
	err     error
}

func (a *mockCallAPI) GetStateAsync(context.Context) (sdk.CallState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.state, a.err
}

func (a *mockCallAPI) DialAsync(context.Context, string) error { return a.act("dial") }

func (a *mockCallAPI) AcceptAsync(context.Context) error { return a.act("accept") }

func (a *mockCallAPI) RejectAsync(context.Context) error { return a.act("reject") }

func (a *mockCallAPI) HangUpAsync(context.Context) error { return a.act("hangup") }

func (a *mockCallAPI) act(s string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, s)

	return a.err
}

func mockGreen(t *testing.T, state string, id string) (*GreenApiCall, *mockCallAPI, *Events) {
	t.Helper()
	s := testSession(true)
	events := NewEvents()
	a := NewGreenApiCall(testConfig(), s, events.Emit)
	mock := &mockCallAPI{state: sdk.CallState{State: state, Info: &sdk.CallInfo{ID: id, WID: s.SenderID}}}
	a.api = mock

	return a, mock, events
}

func TestRejectOfferRequiresMatchingUnansweredCall(t *testing.T) {
	for _, state := range []string{"inc-call", "on-call", "out-call", "idle"} {
		for _, id := range []string{"call-1", "other"} {
			a, m, _ := mockGreen(t, state, id)
			got, err := a.RejectOffer(context.Background())
			want := state == "inc-call" && id == "call-1"

			if err != nil || got != want || len(m.actions) != map[bool]int{true: 1, false: 0}[want] {
				t.Fatalf("%s %s got %v err %v", state, id, got, err)
			}
		}
	}
}

func TestFinishNeverHangsUpAnotherCall(t *testing.T) {
	for _, id := range []string{"call-1", "other"} {
		a, m, _ := mockGreen(t, "on-call", id)
		result, err := a.Finish(context.Background())

		if err != nil {
			t.Fatal(err)
		}

		if id == "other" {
			if result != FinishResultNotOwned || len(m.actions) != 0 {
				t.Fatal("hung up another call")
			}
		} else if result != FinishResultFinished || m.actions[0] != "hangup" {
			t.Fatal("owned call not finished")
		}
	}
}

func TestRemoteEndedCallNotFinishedTwice(t *testing.T) {
	a, m, e := mockGreen(t, "on-call", "call-1")
	a.commandStarted = true
	a.onState(sdk.CallState{State: "idle"})
	_, _ = e.Receive(context.Background(), NewDeadlines())
	result, err := a.Finish(context.Background())

	if err != nil || result != FinishResultAlreadyEnded || len(m.actions) != 0 {
		t.Fatal("double finish")
	}
}

func TestTemporaryDisconnectRecoverablePermanentReported(t *testing.T) {
	a, _, events := mockGreen(t, "on-call", "call-1")
	a.onDisconnect(sdk.DisconnectDetail{Permanent: false})
	a.onDisconnect(sdk.DisconnectDetail{Permanent: true})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := events.Receive(ctx, NewDeadlines())

	if err != nil || e.Kind != "media_failed" {
		t.Fatal("disconnect not reported")
	}

	a.onError(errors.New("duplicate"))
	events.mu.Lock()
	n := len(events.queue)
	events.mu.Unlock()

	if n != 0 {
		t.Fatal("duplicate failure")
	}
}

func TestIncomingCallIDTakesPriorityOverAddress(t *testing.T) {
	a, m, events := mockGreen(t, "on-call", "call-1")
	m.state.Info.WID = "different@lid"
	result, err := a.Accept(context.Background())

	if err != nil || result != AcceptResultAlreadyActive {
		t.Fatal(result, err)
	}

	ctx, c := context.WithTimeout(context.Background(), time.Second)
	defer c()
	e, err := events.Receive(ctx, NewDeadlines())

	if err != nil || e.CallID != "call-1" {
		t.Fatal(e, err)
	}
}

func TestProviderErrorRetainsDetailsWithoutLoggingToken(t *testing.T) {
	a, m, _ := mockGreen(t, "idle", "call-1")
	m.err = errors.New("token=secret")
	err := a.Dial(context.Background())
	var p *ProviderError

	if !errors.As(err, &p) || p.Detail != "token=secret" || err.Error() == p.Detail {
		t.Fatal("error redaction")
	}
}

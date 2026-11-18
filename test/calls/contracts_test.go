package calls_test

import (
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"testing"
	"time"
)

func TestIncomingLifecycleContract(t *testing.T) {
	s, err := calls.NewIncomingCallSession("sender", "call", "en", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	fsm := calls.NewCallStateMachine()
	for _, step := range []struct {
		event calls.CallEvent
		state calls.CallState
	}{
		{calls.CallEventEnqueued, calls.CallStateQueued},
		{calls.CallEventDequeued, calls.CallStateRinging},
		{calls.CallEventBridgeReady, calls.CallStateRinging},
		{calls.CallEventIncomingAccepted, calls.CallStateRinging},
		{calls.CallEventRemoteAccepted, calls.CallStateInCall},
		{calls.CallEventRemoteIdle, calls.CallStateRemoteEnded},
	} {
		tr := fsm.Apply(s, step.event)
		if !tr.Applied || s.State != step.state || tr.Current != s.State {
			t.Fatalf("event %s: transition=%+v state=%s", step.event, tr, s.State)
		}
	}
	if s.QueuedAt.IsZero() || s.StartedAt.IsZero() || s.AnsweredAt.IsZero() || s.EndedAt.IsZero() {
		t.Fatal("lifecycle timestamps missing")
	}
	ended := s.EndedAt
	if fsm.Apply(s, calls.CallEventInternalError).Applied || s.State != calls.CallStateRemoteEnded || s.EndedAt != ended {
		t.Fatal("terminal session changed")
	}
}

func TestPlaybackInterruptionContract(t *testing.T) {
	p := calls.NewPlaybackController(calls.DefaultPlaybackOptions())
	if _, err := p.Append("answer", 0, make([]byte, 960*3)); err != nil {
		t.Fatal(err)
	}
	p.ReadFrame()
	cuts := p.Interrupt()
	if len(cuts) != 1 || cuts[0].ItemID != "answer" || cuts[0].ContentIndex != 0 || cuts[0].PlayedMS != 20 {
		t.Fatalf("interruption must report only heard audio: %+v", cuts)
	}
	for _, b := range p.ReadFrame() {
		if b != 0 {
			t.Fatal("interrupted audio still playing")
		}
	}
	if len(p.Interrupt()) != 0 {
		t.Fatal("repeated interruption reported audio twice")
	}
}

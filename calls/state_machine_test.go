package calls

import (
	"testing"
	"time"
)

func TestCallStateMachineOutgoingLifecycleAndIdempotentTimestamps(t *testing.T) {
	fsm := NewCallStateMachine()
	session := NewCallSession("sender", "chat", "en")
	queued := fsm.Apply(session, CallEventEnqueued)

	if !queued.Applied || queued.Current != CallStateQueued || session.QueuedAt.IsZero() {
		t.Fatalf("enqueue contract failed: %+v session=%+v", queued, session)
	}

	queuedAt := session.QueuedAt
	fsm.Apply(session, CallEventEnqueued)

	if !session.QueuedAt.Equal(queuedAt) {
		t.Fatal("repeated enqueue changed the queue timestamp")
	}

	fsm.Apply(session, CallEventDequeued)

	if session.State != CallStateDialing || session.StartedAt.IsZero() {
		t.Fatalf("outgoing dequeue contract failed: %+v", session)
	}

	fsm.Apply(session, CallEventDialAccepted)
	fsm.Apply(session, CallEventRemoteAccepted)

	if session.State != CallStateConnecting || !session.RemoteAccepted || session.AnsweredAt.IsZero() {
		t.Fatalf("answer contract failed: %+v", session)
	}

	fsm.Apply(session, CallEventBridgeReady)

	if session.State != CallStateInCall || !session.BridgeReady {
		t.Fatalf("bridge contract failed: %+v", session)
	}

	fsm.Apply(session, CallEventTalkTimerExpired)
	fsm.Apply(session, CallEventHangupConfirmed)

	if session.State != CallStateTalkTimeout || session.EndReason != CallEndReasonTalkTimeout || session.EndedAt.IsZero() || !session.Finished() {
		t.Fatalf("hangup contract failed: %+v", session)
	}
}

func TestCallStateMachineIncomingRulesAndTransitionDetails(t *testing.T) {
	fsm := NewCallStateMachine()
	deadline := time.Now().Add(time.Minute)
	session, err := NewIncomingCallSession("sender", "call", "en", deadline)

	if err != nil {
		t.Fatal(err)
	}

	fsm.Apply(session, CallEventEnqueued)
	fsm.Apply(session, CallEventDequeued)

	if session.State != CallStateRinging {
		t.Fatalf("incoming dequeue must ring: %s", session.State)
	}

	if transition := fsm.Apply(session, CallEventIncomingAccepted); !transition.Applied || !session.IncomingAcceptCompleted {
		t.Fatalf("incoming accept was not recorded: %+v %+v", transition, session)
	}

	if transition := fsm.Apply(session, CallEventRemoteAccepted); !transition.Applied || session.State != CallStateConnecting {
		t.Fatalf("remote answer transition failed: %+v %+v", transition, session)
	}

	remoteReason, code := "remote-info", "E123"
	fsm.Apply(session, CallEventInternalError, TransitionDetails{RemoteReason: &remoteReason, ErrorCode: &code})

	if session.State != CallStateFailed || session.EndReason != CallEndReasonError || session.RemoteReason != remoteReason || session.ErrorCode != code || session.EndedAt.IsZero() {
		t.Fatalf("error details were not committed: %+v", session)
	}
}

func TestCallStateMachineIgnoresInvalidDirectionAndUnknownEvents(t *testing.T) {
	fsm := NewCallStateMachine()
	session := NewCallSession("sender", "chat", "en")
	transition := fsm.Apply(session, CallEventIncomingExpired)

	if transition.Applied || transition.Changed() || transition.Outcome() != "ignored" || session.State != CallStateTriggered {
		t.Fatalf("invalid transition changed state: %+v %+v", transition, session)
	}

	transition = fsm.Apply(session, CallEvent("unknown"))

	if transition.Applied || session.State != CallStateTriggered {
		t.Fatalf("unknown event changed state: %+v", transition)
	}
}

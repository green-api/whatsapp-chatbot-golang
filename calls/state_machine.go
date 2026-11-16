package calls

import "time"

type stateChange struct {
	Target                                      CallState
	Reason                                      CallEndReason
	Queued, Started, Answered, Bridge, Accepted bool
}

type transitionRule struct {
	change    stateChange
	Resolver  func(*CallSession) stateChange
	Direction CallDirection
}

type transitionKey struct {
	State CallState
	Event CallEvent
}

func dequeue(s *CallSession) stateChange {
	target := CallStateDialing

	if s.Direction == CallDirectionIncoming {
		target = CallStateRinging
	}

	return stateChange{Target: target, Started: true}
}

func answer(s *CallSession) stateChange {
	target := CallStateConnecting

	if s.BridgeReady {
		target = CallStateInCall
	}

	return stateChange{Target: target, Answered: true}
}

func bridge(s *CallSession) stateChange {
	target := s.State

	if s.RemoteAccepted {
		target = CallStateInCall
	}

	return stateChange{Target: target, Bridge: true}
}

func idle(s *CallSession) stateChange {
	if s.RemoteAccepted {
		return stateChange{Target: CallStateRemoteEnded, Reason: CallEndReasonRemoteHangup}
	}

	return stateChange{Target: CallStateRejected, Reason: CallEndReasonRemoteRejected}
}

func hangup(s *CallSession) stateChange {
	target := map[CallEndReason]CallState{CallEndReasonRingTimeout: CallStateRingTimeout, CallEndReasonTalkTimeout: CallStateTalkTimeout, CallEndReasonShutdown: CallStateCancelled, CallEndReasonUserCancelled: CallStateCancelled}[s.EndReason]

	if target == "" {
		target = CallStateFailed
	}

	return stateChange{Target: target}
}

func rules() map[transitionKey]transitionRule {
	r := map[transitionKey]transitionRule{
		{CallStateTriggered, CallEventEnqueued}:      {change: stateChange{Target: CallStateQueued, Queued: true}},
		{CallStateQueued, CallEventDequeued}:         {Resolver: dequeue},
		{CallStateDialing, CallEventDialAccepted}:    {change: stateChange{Target: CallStateRinging}},
		{CallStateEnding, CallEventHangupConfirmed}:  {Resolver: hangup},
		{CallStateInCall, CallEventTalkTimerExpired}: {change: stateChange{Target: CallStateEnding, Reason: CallEndReasonTalkTimeout}},
	}

	for _, state := range []CallState{CallStateDialing, CallStateRinging, CallStateConnecting, CallStateInCall} {
		r[transitionKey{state, CallEventBridgeReady}] = transitionRule{Resolver: bridge}
		r[transitionKey{state, CallEventRemoteIdle}] = transitionRule{Resolver: idle}
		r[transitionKey{state, CallEventIncomingAccepted}] = transitionRule{change: stateChange{Target: state, Accepted: true}, Direction: CallDirectionIncoming}
	}

	for _, state := range []CallState{CallStateDialing, CallStateRinging} {
		r[transitionKey{state, CallEventRemoteAccepted}] = transitionRule{Resolver: answer}
		r[transitionKey{state, CallEventRingTimerExpired}] = transitionRule{change: stateChange{Target: CallStateEnding, Reason: CallEndReasonRingTimeout}}
	}

	for _, state := range []CallState{CallStateTriggered, CallStateQueued, CallStateDialing, CallStateRinging, CallStateConnecting, CallStateInCall, CallStateEnding} {
		for event, reason := range map[CallEvent]CallEndReason{CallEventInternalError: CallEndReasonError, CallEventShutdownRequested: CallEndReasonShutdown, CallEventCancelRequested: CallEndReasonUserCancelled} {
			target := CallStateCancelled

			if event == CallEventInternalError {
				target = CallStateFailed
			}

			r[transitionKey{state, event}] = transitionRule{change: stateChange{Target: target, Reason: reason}}
		}
	}

	for _, state := range []CallState{CallStateTriggered, CallStateQueued, CallStateRinging} {
		for event, reason := range map[CallEvent]CallEndReason{CallEventIncomingExpired: CallEndReasonIncomingExpired, CallEventCallerLeft: CallEndReasonCallerLeft} {
			r[transitionKey{state, event}] = transitionRule{change: stateChange{Target: CallStateRejected, Reason: reason}, Direction: CallDirectionIncoming}
		}
	}

	return r
}

var callRules = rules()

type CallStateMachine struct{}

func NewCallStateMachine() *CallStateMachine { return &CallStateMachine{} }

func (f *CallStateMachine) Apply(s *CallSession, event CallEvent, details ...TransitionDetails) StateTransition {
	r, ok := f.findRule(s, event)

	if !ok {
		return StateTransition{s.State, s.State, event, false}
	}

	change := r.change

	if r.Resolver != nil {
		change = r.Resolver(s)
	}

	var d TransitionDetails

	if len(details) > 0 {
		d = details[0]
	}

	return f.commit(s, event, change, time.Now().UTC(), d)
}

func (f *CallStateMachine) findRule(s *CallSession, e CallEvent) (transitionRule, bool) {
	r, ok := callRules[transitionKey{s.State, e}]

	return r, ok && (r.Direction == "" || r.Direction == s.Direction)
}

func (f *CallStateMachine) commit(s *CallSession, e CallEvent, c stateChange, now time.Time, d TransitionDetails) StateTransition {
	prev := s.State
	s.State = c.Target

	if c.Reason != "" {
		s.EndReason = c.Reason
	}

	if d.RemoteReason != nil {
		s.RemoteReason = *d.RemoteReason
	}

	if d.ErrorCode != nil {
		s.ErrorCode = *d.ErrorCode
	}

	if c.Queued && s.QueuedAt.IsZero() {
		s.QueuedAt = now
	}

	if c.Started && s.StartedAt.IsZero() {
		s.StartedAt = now
	}

	if c.Answered {
		s.RemoteAccepted = true

		if s.AnsweredAt.IsZero() {
			s.AnsweredAt = now
		}
	}

	s.BridgeReady = s.BridgeReady || c.Bridge
	s.IncomingAcceptCompleted = s.IncomingAcceptCompleted || c.Accepted

	if s.Finished() && s.EndedAt.IsZero() {
		s.EndedAt = now
	}

	return StateTransition{prev, s.State, e, true}
}

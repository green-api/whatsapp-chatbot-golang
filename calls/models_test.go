package calls

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCallSessionConstructionAndValidation(t *testing.T) {
	session := NewCallSession("sender", "chat", "en")

	if session.Direction != CallDirectionOutgoing || session.State != CallStateTriggered || session.CreatedAt.IsZero() {
		t.Fatalf("unexpected outgoing session defaults: %+v", session)
	}

	if len(session.SessionID) != 36 || session.SessionID[14] != '4' {
		t.Fatalf("session ID is not a UUID v4: %q", session.SessionID)
	}

	deadline := time.Now().Add(time.Minute)
	incoming, err := NewIncomingCallSession("sender", "call-1", "en", deadline)

	if err != nil {
		t.Fatal(err)
	}

	if incoming.Direction != CallDirectionIncoming || incoming.ChatID != "sender" || incoming.CallID != "call-1" || !incoming.IncomingDeadline.Equal(deadline) {
		t.Fatalf("unexpected incoming session: %+v", incoming)
	}

	if _, err := NewIncomingCallSession("", "call-1", "en", deadline); err == nil {
		t.Fatal("empty sender accepted")
	}

	if _, err := NewIncomingCallSession("sender", "", "en", deadline); err == nil {
		t.Fatal("empty call ID accepted")
	}

	if _, err := NewIncomingCallSession("sender", "call-1", "en", time.Time{}); err == nil {
		t.Fatal("zero deadline accepted")
	}

	blank := " \t"
	session.Prompt = &blank

	if err := session.Validate(); err == nil {
		t.Fatal("blank prompt accepted")
	}

	valid := "hello"
	session.Prompt = &valid

	if err := session.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCallSessionFinishedStates(t *testing.T) {
	finished := []CallState{CallStateCancelled, CallStateRejected, CallStateRingTimeout, CallStateRemoteEnded, CallStateTalkTimeout, CallStateFailed}

	for _, state := range finished {
		t.Run(string(state), func(t *testing.T) {
			if !(&CallSession{State: state}).Finished() {
				t.Fatalf("%q should be finished", state)
			}
		})
	}

	for _, state := range []CallState{CallStateTriggered, CallStateQueued, CallStateDialing, CallStateRinging, CallStateConnecting, CallStateInCall, CallStateEnding} {
		if (&CallSession{State: state}).Finished() {
			t.Fatalf("%q should remain active", state)
		}
	}
}

func TestCallExecutionResultRecordingOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "call.mp3")

	if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}

	result := NewCallExecutionResult(path)

	if result.RecordingPath() != path || result.TakeRecording() != path || result.RecordingPath() != "" {
		t.Fatal("TakeRecording did not transfer ownership")
	}

	if err := result.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("transferred recording was removed: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	ownedPath := filepath.Join(t.TempDir(), "owned.mp3")

	if err := os.WriteFile(ownedPath, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := NewCallExecutionResult(ownedPath).Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(ownedPath); !os.IsNotExist(err) {
		t.Fatalf("Close retained owned recording: %v", err)
	}
}

func TestProviderErrorsRedactCredentialsAndRetainIdentity(t *testing.T) {
	config := ProviderConfig{OpenAIAPIKey: "secret-token"}

	if strings.Contains(config.String(), "secret-token") || strings.Contains(config.GoString(), "secret-token") {
		t.Fatal("provider config string leaked credential")
	}

	err := (&ProviderError{Provider: "openai", Code: "rate_limit", Detail: "secret-token"}).Error()

	if strings.Contains(err, "secret-token") || !strings.Contains(err, "rate_limit") {
		t.Fatalf("unexpected provider error string: %q", err)
	}
}

func TestErrorCodeClassification(t *testing.T) {
	for _, test := range []struct {
		err  error
		code string
	}{
		{validationError("invalid options"), "validation_error"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New("operation failed"), "runtime_error"},
		{&ProviderError{Provider: "custom", Code: "rejected"}, "rejected"},
		{&CallCleanupError{Stage: "voice_close", Cause: validationError("invalid")}, "cleanup_voice_close_validation_error"},
	} {
		if code := errorCode(test.err); code != test.code {
			t.Fatalf("error code = %q, want %q", code, test.code)
		}
	}
}

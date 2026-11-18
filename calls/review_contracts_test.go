package calls

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCallbackStateControlsAdmission(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue", true: "end"}[terminal], func(t *testing.T) {
			s := testSession(true)
			var runner *callRunner
			callback := func(s *CallSession, event CallEvent, _ ...TransitionDetails) StateTransition {
				previous := s.State
				if !runner.NotifyIncomingEnded(s.CallID) {
					t.Fatal("notification lost inside callback")
				}
				if terminal {
					s.State = CallStateRemoteEnded
				}
				return StateTransition{Previous: previous, Current: s.State, Event: event, Applied: true}
			}
			runner = newCallRunner(s, callback, testConfig(), testOptions(), newScenario().dependencies())
			defer runner.events.Close()
			// The built-in FSM ends on InternalError; this callback may deliberately continue.
			runner.apply(CallEventInternalError)
			if runner.NotifyIncomingEnded(s.CallID) == terminal {
				t.Fatal("admission disagrees with actual state")
			}
			if terminal {
				if err := runner.receiveEvents(context.Background()); err != nil {
					t.Fatal(err)
				}
			} else {
				event, err := runner.events.Receive(context.Background(), runner.deadlines)
				if err != nil || event.Kind != "caller_left" {
					t.Fatal(event, err)
				}
			}
		})
	}
}

func TestExecutionErrorTransitionsOnce(t *testing.T) {
	sc := newScenario()
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), testOptions(), sc.dependencies())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	count := 0
	callback := func(s *CallSession, e CallEvent, d ...TransitionDetails) StateTransition {
		if e == CallEventShutdownRequested || e == CallEventInternalError {
			count++
		}
		return NewCallStateMachine().Apply(s, e, d...)
	}
	runner := newCallRunner(testSession(false), callback, service.config, service.options, sc.dependencies())
	cancel()
	_, err = runner.Run(ctx)
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("err=%v transitions=%d", err, count)
	}
}

func TestRealtimeEventJSONFields(t *testing.T) {
	var event realtimeEvent
	err := json.Unmarshal([]byte(`{"type":"error","response_id":"r","item_id":"i","delta":"audio","content_index":2,"response":{"id":"r","status":"failed"},"error":{"code":"bad","message":"detail","event_id":"cancel"}}`), &event)
	if err != nil {
		t.Fatal(err)
	}
	if event.ResponseID != "r" || event.ItemID != "i" || event.ContentIndex != 2 || event.Delta != "audio" || event.Response.Status != "failed" || event.Error.EventID != "cancel" {
		t.Fatalf("fields not decoded: %+v", event)
	}
}

func TestEncoderFailureRetainsDiagnosticsAndCause(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper requires Unix")
	}
	directory := t.TempDir()
	encoder := filepath.Join(directory, "encoder")
	if err := os.WriteFile(encoder, []byte("#!/bin/sh\nprintf 'unsupported codec' >&2\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	options := DefaultRecordingOptions()
	options.Directory, options.EncoderPath = directory, encoder
	recorder := NewCallRecorder(options)
	if err := recorder.Write(TrackCaller, PcmChunk{Data: []byte{1, 0}}); err != nil {
		t.Fatal(err)
	}
	path, err := recorder.Finish(context.Background())
	var recording *RecordingError
	var exit *exec.ExitError
	if path != "" || !errors.As(err, &recording) || !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("path=%q error=%v", path, err)
	}
	if !strings.Contains(recording.Diagnostic, "unsupported codec") {
		t.Fatal("encoder diagnostic lost")
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files leaked: %v %v", files, err)
	}
}

func TestEncoderDiagnosticsAreBounded(t *testing.T) {
	d := &encoderDiagnostics{}
	for i := 0; i < 3; i++ {
		n, err := d.Write(make([]byte, 3000))
		if n != 3000 || err != nil {
			t.Fatal("stderr must be drained")
		}
	}
	if len(d.data) != 4096 {
		t.Fatal("stderr exceeded limit")
	}
}

func TestCaptureFailureReachesFinish(t *testing.T) {
	r := NewCallRecorder(DefaultRecordingOptions())
	cause := errors.New("capture write failed")
	r.capture.err = cause
	_, err := r.Finish(context.Background())
	if !errors.Is(err, cause) {
		t.Fatalf("capture cause lost: %v", err)
	}
}

func TestBrokenRecordingDoesNotFailConversation(t *testing.T) {
	sc := newScenario()
	options := testOptions()
	options.Recording.Enabled = true
	options.Recording.Directory = t.TempDir()
	options.Recording.EncoderPath = filepath.Join(t.TempDir(), "missing-encoder")
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), options, sc.dependencies())
	if err != nil {
		t.Fatal(err)
	}
	callback := func(s *CallSession, e CallEvent, details ...TransitionDetails) StateTransition {
		if e == CallEventBridgeReady {
			if err := service.runner.resources.Recorder.Write(TrackCaller, PcmChunk{Data: []byte{1, 0}}); err != nil {
				t.Fatal(err)
			}
		}
		return NewCallStateMachine().Apply(s, e, details...)
	}
	session := testSession(false)
	result, err := service.Execute(context.Background(), session, callback)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if session.State != CallStateRemoteEnded || result.RecordingPath() != "" {
		t.Fatal("optional export changed call outcome")
	}
	var recording *RecordingError
	if !errors.As(result.RecordingError(), &recording) || !errors.Is(result.RecordingError(), os.ErrNotExist) {
		t.Fatalf("recording failure not returned: %v", result.RecordingError())
	}
	files, err := os.ReadDir(options.Recording.Directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("recording files leaked: %v %v", files, err)
	}
}

func TestExecuteWithoutRecordingFailure(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		sc := newScenario()
		options := testOptions()
		options.Recording.Enabled = enabled
		options.Recording.Directory = t.TempDir()
		service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), options, sc.dependencies())
		if err != nil {
			t.Fatal(err)
		}
		session := testSession(false)
		result, err := service.Execute(context.Background(), session, NewCallStateMachine().Apply)
		if err != nil {
			t.Fatal(err)
		}
		if result.RecordingPath() != "" || result.RecordingError() != nil {
			t.Fatal("disabled or empty recording reported a failure")
		}
		if err := result.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExecuteReportsRecordingTimeoutAsSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell encoder requires Unix")
	}
	encoder := filepath.Join(t.TempDir(), "encoder")
	script := "#!/bin/sh\ncat >/dev/null\n: > \"$0.started\"\nwhile [ ! -f \"$0.release\" ]; do sleep 0.01; done\n"
	if err := os.WriteFile(encoder, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(encoder+".release", nil, 0600)
	options := testOptions()
	options.Recording.Enabled = true
	options.Recording.Directory, options.Recording.EncoderPath = t.TempDir(), encoder
	service, err := NewWhatsAppCallServiceWithDependencies(testConfig(), options, newScenario().dependencies())
	if err != nil {
		t.Fatal(err)
	}
	var recorder *CallRecorder
	callback := func(s *CallSession, e CallEvent, d ...TransitionDetails) StateTransition {
		if e == CallEventBridgeReady {
			recorder = service.runner.resources.Recorder
			if err := recorder.Write(TrackCaller, PcmChunk{Data: []byte{1, 0}}); err != nil {
				t.Fatal(err)
			}
		}
		return NewCallStateMachine().Apply(s, e, d...)
	}
	result, err := service.Execute(context.Background(), testSession(false), callback)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if !errors.Is(result.RecordingError(), context.DeadlineExceeded) || result.RecordingPath() != "" {
		t.Fatal("export timeout not reported separately")
	}
	waitForEncoderStart(t, encoder+".started")
	if err := os.WriteFile(encoder+".release", nil, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if path, err := recorder.Finish(ctx); err != nil || path != "" {
		t.Fatalf("late export retained: %q %v", path, err)
	}
	if !errors.Is(result.RecordingError(), context.DeadlineExceeded) {
		t.Fatal("late completion changed error snapshot")
	}
	files, err := os.ReadDir(options.Recording.Directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("late recording files leaked: %v %v", files, err)
	}
}

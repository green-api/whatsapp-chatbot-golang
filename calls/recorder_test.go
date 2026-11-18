package calls

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResultContextReleasesOrTransfersRecording(t *testing.T) {
	for _, take := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "recording.mp3")
		os.WriteFile(path, []byte{1}, 0600)
		r := NewCallExecutionResult(path)

		if take && r.TakeRecording() != path {
			t.Fatal("wrong ownership transfer")
		}

		if err := r.Close(); err != nil {
			t.Fatal(err)
		}

		_, err := os.Stat(path)

		if take && err != nil || !take && !errors.Is(err, os.ErrNotExist) {
			t.Fatal("ownership mismatch")
		}
	}
}

func TestPcmTracksKeepGapAndWriterIsBounded(t *testing.T) {
	o := DefaultRecordingOptions()
	o.Directory = t.TempDir()
	capture := NewPcmCapture(o)
	data := []byte{7, 0}
	capture.Write(TrackCaller, PcmChunk{100, data})
	tracks, err := capture.Seal()

	if err != nil {
		t.Fatal(err)
	}

	defer capture.Close()
	raw := make([]byte, 202)
	tracks[TrackCaller].ReadAt(raw, 0)

	if raw[0] != 0 || raw[200] != 7 {
		t.Fatal("gap not preserved")
	}

	if capture.Write(TrackCaller, PcmChunk{0, data}) == nil {
		t.Fatal("sealed capture reopened")
	}
}

func TestNoAudioOrBrokenEncoderReturnsNoRecording(t *testing.T) {
	o := DefaultRecordingOptions()
	o.Directory = t.TempDir()
	o.EncoderPath = "/nonexistent/call-encoder"

	for _, audio := range []bool{false, true} {
		r := NewCallRecorder(o)

		if audio {
			r.Write(TrackCaller, PcmChunk{0, []byte{1, 0}})
		}

		path, err := r.Finish(context.Background())

		if path != "" || (err != nil) != audio {
			t.Fatal("recording failure must be observable", path, err)
		}

		files, _ := os.ReadDir(o.Directory)

		if len(files) != 0 {
			t.Fatal("failed recording leaked files")
		}
	}
}

func TestTakeReadyOrDiscardPreventsPendingExport(t *testing.T) {
	options := DefaultRecordingOptions()
	options.Directory = t.TempDir()
	options.EncoderPath = filepath.Join(t.TempDir(), "missing-encoder")
	recorder := NewCallRecorder(options)
	if err := recorder.Write(TrackCaller, PcmChunk{Data: []byte{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if path := recorder.TakeReadyOrDiscard(); path != "" {
		t.Fatal("unfinished recording transferred")
	}
	path, err := recorder.Finish(context.Background())
	if err != nil || path != "" {
		t.Fatalf("discarded capture exported: %q %v", path, err)
	}
	files, err := os.ReadDir(options.Directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("discarded recording leaked files: %v %v", files, err)
	}
}

func TestTakeReadyOrDiscardTransfersReadyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	recorder := NewCallRecorder(DefaultRecordingOptions())
	recorder.path = path
	if got := recorder.TakeReadyOrDiscard(); got != path {
		t.Fatalf("transferred %q", got)
	}
	recorder.Discard()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("transferred recording removed", err)
	}
	if got := recorder.TakeReadyOrDiscard(); got != "" {
		t.Fatal("recording transferred twice")
	}
}

func TestTakeReadyOrDiscardRemovesLateExport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell encoder requires Unix")
	}
	encoder := filepath.Join(t.TempDir(), "encoder")
	script := "#!/bin/sh\ncat >/dev/null\n: > \"$0.started\"\nwhile [ ! -f \"$0.release\" ]; do sleep 0.01; done\n"
	if err := os.WriteFile(encoder, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(encoder+".release", nil, 0600)
	options := DefaultRecordingOptions()
	options.Directory, options.EncoderPath = t.TempDir(), encoder
	recorder := NewCallRecorder(options)
	if err := recorder.Write(TrackCaller, PcmChunk{Data: []byte{1, 0}}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		path, err := recorder.Finish(context.Background())
		if path != "" {
			err = errors.Join(err, errors.New("late recording retained"))
		}
		done <- err
	}()
	waitForEncoderStart(t, encoder+".started")
	if recorder.TakeReadyOrDiscard() != "" {
		t.Fatal("unfinished export transferred")
	}
	if err := os.WriteFile(encoder+".release", nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("export did not complete")
	}
	files, err := os.ReadDir(options.Directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("late export leaked files: %v %v", files, err)
	}
}

func waitForEncoderStart(t *testing.T, path string) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("encoder did not start")
		case <-tick.C:
		}
	}
}

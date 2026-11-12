package calls

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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

		if err != nil || path != "" {
			t.Fatal("optional recorder failed call", path, err)
		}

		files, _ := os.ReadDir(o.Directory)

		if len(files) != 0 {
			t.Fatal("failed recording leaked files")
		}
	}
}

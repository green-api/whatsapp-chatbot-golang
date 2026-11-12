package calls_test

import (
	"encoding/hex"
	"encoding/json"
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestPythonFSMReference(t *testing.T) {
	data, err := os.ReadFile("testdata/python_fsm.json")

	if err != nil {
		t.Fatal(err)
	}

	var rows []struct {
		Direction                        calls.CallDirection
		State                            calls.CallState
		Event                            calls.CallEvent
		Accepted, Bridge                 bool
		Reason                           calls.CallEndReason
		Current                          calls.CallState
		EndReason                        calls.CallEndReason `json:"end_reason"`
		Applied                          bool
		RemoteAccepted                   bool `json:"remote_accepted"`
		BridgeReady                      bool `json:"bridge_ready"`
		IncomingAcceptCompleted          bool `json:"incoming_accept_completed"`
		Queued, Started, Answered, Ended bool
	}

	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}

	for i, r := range rows {
		s := calls.NewCallSession("s", "s", "ru")
		s.Direction = r.Direction
		s.State = r.State
		s.RemoteAccepted = r.Accepted
		s.BridgeReady = r.Bridge
		s.EndReason = r.Reason
		tr := calls.NewCallStateMachine().Apply(s, r.Event)

		if s.State != r.Current || s.EndReason != r.EndReason || tr.Applied != r.Applied || s.RemoteAccepted != r.RemoteAccepted || s.BridgeReady != r.BridgeReady || s.IncomingAcceptCompleted != r.IncomingAcceptCompleted || !s.QueuedAt.IsZero() != r.Queued || !s.StartedAt.IsZero() != r.Started || !s.AnsweredAt.IsZero() != r.Answered || !s.EndedAt.IsZero() != r.Ended {
			t.Fatalf("Python fixture %d (%s %s %s): session=%+v transition=%+v", i, r.Direction, r.State, r.Event, s, tr)
		}
	}

	t.Logf("Compared %d Python transitions", len(rows))
}

func TestPythonPlaybackReference(t *testing.T) {
	data, err := os.ReadFile("testdata/python_playback.json")

	if err != nil {
		t.Fatal(err)
	}

	var rows []struct {
		Policy calls.PlaybackOverflowPolicy
		Count  int
		Result calls.AppendResult
		States []calls.PlaybackState
		Frames []string
		Cuts   []struct {
			ItemID       string `json:"item_id"`
			ContentIndex int    `json:"content_index"`
			PlayedMS     int    `json:"played_ms"`
		}
		Dropped   int
		Deletions []string
	}

	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}

	for i, r := range rows {
		o := calls.DefaultPlaybackOptions()
		o.SoftLimitSeconds = .02
		o.HardLimitSeconds = .06
		o.OverflowPolicy = r.Policy
		p := calls.NewPlaybackController(o)
		pcm := make([]byte, r.Count*2)

		for j := 0; j < len(pcm); j += 2 {
			pcm[j] = 1
		}

		result, err := p.Append("item", 0, pcm)

		if err != nil || result != r.Result {
			t.Fatalf("trace %d append %s %v", i, result, err)
		}

		p.FinishResponse()

		if p.State() != r.States[0] {
			t.Fatal("initial state")
		}

		for j, want := range r.Frames {
			frame := p.ReadFrame()

			if hex.EncodeToString(frame) != want || p.State() != r.States[j+1] {
				t.Fatalf("trace %d frame %d differs", i, j)
			}
		}

		cuts := p.Interrupt()

		if len(cuts) != len(r.Cuts) {
			t.Fatalf("trace %d cuts %+v expected %+v", i, cuts, r.Cuts)
		}

		for j, c := range cuts {
			if c.ItemID != r.Cuts[j].ItemID || c.PlayedMS != r.Cuts[j].PlayedMS {
				t.Fatal("cut differs")
			}
		}

		deletions := p.TakeItemsToDelete()
		sort.Strings(deletions)

		if !reflect.DeepEqual(deletions, r.Deletions) || p.DroppedSamples() != r.Dropped {
			t.Fatalf("trace %d deletion/drop differs", i)
		}
	}
}

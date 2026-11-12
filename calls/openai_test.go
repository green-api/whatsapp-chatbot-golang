package calls

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionSettingsVadAndNoiseReduction(t *testing.T) {
	for _, noise := range []string{"near_field", "far_field", ""} {
		vad := DefaultVadOptions()
		vad.NoiseReduction = noise
		v := NewRealtimeConversation(testConfig(), DefaultPlaybackOptions(), func(RuntimeEvent) bool { return true }, "ru", 1, vad)
		settings := v.SessionSettings()
		input := settings["audio"].(map[string]any)["input"].(map[string]any)

		if noise == "" && input["noise_reduction"] != nil {
			t.Fatal("noise not disabled")
		}

		if input["turn_detection"].(map[string]any)["threshold"] != .7 {
			t.Fatal("VAD not passed")
		}

		v.Close()
	}
}

func TestRealtimeWebsocketConfigurationPromptPCMAndInterruption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	received := make(chan map[string]any, 30)
	send := make(chan map[string]any, 30)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)

		if err != nil {
			return
		}

		defer conn.CloseNow()
		done := make(chan struct{})

		go func() {
			defer close(done)

			for {
				_, b, err := conn.Read(ctx)

				if err != nil {
					return
				}

				var e map[string]any
				json.Unmarshal(b, &e)
				received <- e

				if e["type"] == "session.update" {
					send <- map[string]any{"type": "session.updated"}
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case e := <-send:
				b, _ := json.Marshal(e)

				if conn.Write(ctx, websocket.MessageText, b) != nil {
					return
				}
			}
		}
	}))
	defer server.Close()
	config := testConfig()
	config.RealtimeURL = "ws" + strings.TrimPrefix(server.URL, "http")
	events := NewEvents()
	v := NewRealtimeConversation(config, DefaultPlaybackOptions(), events.Emit, "ru", 1, DefaultVadOptions())
	defer v.Close()

	if err := v.Start(ctx); err != nil {
		t.Fatal(err)
	}

	next := func(want string) map[string]any {
		t.Helper()

		select {
		case e := <-received:
			if e["type"] != want {
				t.Fatalf("want %s got %v", want, e)
			}

			return e
		case <-ctx.Done():
			t.Fatal("server event timeout")

			return nil
		}
	}
	next("session.update")
	prompt := "Exact prompt"

	if err := v.PromptOnce(ctx, &prompt); err != nil {
		t.Fatal(err)
	}

	if next("response.create")["response"].(map[string]any)["instructions"] != prompt {
		t.Fatal("prompt changed")
	}

	v.PromptOnce(ctx, nil)

	if err := v.WriteAudio(ctx, []byte{1, 2}); err != nil {
		t.Fatal(err)
	}

	if next("input_audio_buffer.append")["audio"] != base64.StdEncoding.EncodeToString([]byte{1, 2}) {
		t.Fatal("PCM not encoded")
	}

	send <- map[string]any{"type": "response.created", "response": map[string]any{"id": "r"}}
	send <- map[string]any{"type": "response.output_audio.delta", "response_id": "r", "item_id": "i", "content_index": 0, "delta": base64.StdEncoding.EncodeToString(make([]byte, 960*3))}
	deadline := time.Now().Add(time.Second)

	for v.playback.BufferedBytes() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("audio not received")
		}

		time.Sleep(time.Millisecond)
	}

	v.ReadFrame()
	send <- map[string]any{"type": "input_audio_buffer.speech_started"}
	next("response.cancel")
	cut := next("conversation.item.truncate")

	if cut["audio_end_ms"] != float64(20) {
		t.Fatal("cut not based on heard audio", cut)
	}

	if v.playback.BufferedBytes() != 0 {
		t.Fatal("unheard audio not discarded")
	}
}

func TestGreetingAndVadValidation(t *testing.T) {
	v := NewRealtimeConversation(testConfig(), DefaultPlaybackOptions(), func(RuntimeEvent) bool { return true }, "adaptive", 1, DefaultVadOptions())

	if !strings.Contains(v.defaultGreetingInstruction(), "naturally switch") {
		t.Fatal("adaptive greeting")
	}

	for _, threshold := range []float64{-1, 2} {
		vad := DefaultVadOptions()
		vad.Threshold = threshold

		if vad.Validate() == nil {
			t.Fatal("invalid threshold accepted")
		}
	}

	v.Close()
}

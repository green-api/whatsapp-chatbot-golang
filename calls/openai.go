package calls

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// RealtimeConversation follows calls/openai.py; playback is owned by PlaybackController.
type RealtimeConversation struct {
	mu                   sync.Mutex
	config               ProviderConfig
	options              PlaybackOptions
	vad                  VadOptions
	emit                 EventSink
	language             string
	playback             *PlaybackController
	conn                 *websocket.Conn
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	closed, greeted      bool
	responses, cancelled map[string]bool
	responseItems        map[string]map[AudioItemKey]bool
	cancelEvents         map[string]time.Time
	droppedAudio         map[string]int
	shutdown             float64
}

func NewRealtimeConversation(c ProviderConfig, p PlaybackOptions, e EventSink, language string, shutdown float64, vad VadOptions) *RealtimeConversation {
	ctx, cancel := context.WithCancel(context.Background())

	return &RealtimeConversation{config: c, options: p, vad: vad, emit: e, language: language, playback: NewPlaybackController(p), ctx: ctx, cancel: cancel, responses: map[string]bool{}, cancelled: map[string]bool{}, responseItems: map[string]map[AudioItemKey]bool{}, cancelEvents: map[string]time.Time{}, droppedAudio: map[string]int{}, shutdown: shutdown}
}

func (v *RealtimeConversation) SessionSettings() map[string]any {
	var noise any

	if v.vad.NoiseReduction != "" {
		noise = map[string]any{"type": v.vad.NoiseReduction}
	}

	s := map[string]any{"type": "realtime", "output_modalities": []string{"audio"}, "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": PCMRate}, "noise_reduction": noise, "turn_detection": map[string]any{"type": "server_vad", "threshold": v.vad.Threshold, "prefix_padding_ms": v.vad.PrefixPaddingMS, "silence_duration_ms": v.vad.SilenceDurationMS, "create_response": true, "interrupt_response": true}}, "output": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": PCMRate}, "voice": v.config.RealtimeVoice}}}

	if v.config.Instructions != nil {
		s["instructions"] = *v.config.Instructions
	}

	return s
}

func (v *RealtimeConversation) openConnection(ctx context.Context) (*websocket.Conn, error) {
	endpoint := v.config.RealtimeURL

	if endpoint == "" {
		endpoint = "wss://api.openai.com/v1/realtime"
	}

	u, err := url.Parse(endpoint)

	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("model", v.config.RealtimeModel)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + v.config.OpenAIAPIKey}}})

	if err != nil {
		return nil, wrapProviderError("openai", "connect_failed", err)
	}

	return conn, nil
}

func (v *RealtimeConversation) Start(ctx context.Context) error {
	conn, err := v.openConnection(ctx)

	if err != nil {
		return err
	}

	conn.SetReadLimit(16 << 20)
	v.mu.Lock()

	if v.closed {
		v.mu.Unlock()
		conn.CloseNow()

		return errors.New("Realtime conversation is closing")
	}

	v.conn = conn
	v.mu.Unlock()

	if err = v.send(ctx, map[string]any{"type": "session.update", "session": v.SessionSettings()}); err != nil {
		return err
	}

	for {
		var e realtimeEvent

		if err = v.read(ctx, &e); err != nil {
			return err
		}

		if e.Type == "error" {
			return providerError(e)
		}

		if e.Type == "session.updated" {
			break
		}
	}

	v.spawn(func(ctx context.Context) error {
		for {
			var e realtimeEvent

			if err := v.read(ctx, &e); err != nil {
				return err
			}

			if err := v.dispatch(ctx, e); err != nil {
				return err
			}
		}
	})
	v.spawn(v.watchPlayback)

	return nil
}

func (v *RealtimeConversation) send(ctx context.Context, e map[string]any) error {
	v.mu.Lock()
	c, closed := v.conn, v.closed
	v.mu.Unlock()

	if closed || c == nil {
		return errors.New("Realtime conversation is closing")
	}

	b, err := json.Marshal(e)

	if err != nil {
		return err
	}

	return c.Write(ctx, websocket.MessageText, b)
}

func (v *RealtimeConversation) read(ctx context.Context, e *realtimeEvent) error {
	v.mu.Lock()
	c := v.conn
	v.mu.Unlock()
	_, data, err := c.Read(ctx)

	if err != nil {
		return err
	}

	return json.Unmarshal(data, e)
}

func (v *RealtimeConversation) spawn(fn func(context.Context) error) {
	v.mu.Lock()

	if v.closed {
		v.mu.Unlock()

		return
	}

	v.wg.Add(1)
	v.mu.Unlock()

	go func() {
		defer v.wg.Done()

		if err := fn(v.ctx); err != nil && v.ctx.Err() == nil {
			v.emit(RuntimeEvent{Kind: "media_failed", Error: err})
		}
	}()
}

func (v *RealtimeConversation) ReadFrame() []byte {
	frame := v.playback.ReadFrame()
	cuts := v.playback.TakeDrainedCuts()
	items := v.playback.TakeItemsToDelete()

	if len(cuts) > 0 || len(items) > 0 {
		v.spawn(func(ctx context.Context) error {
			if err := v.truncateCuts(ctx, cuts); err != nil {
				return err
			}

			return v.deleteItems(ctx, items)
		})
	}

	return frame
}

func (v *RealtimeConversation) ReplaceTrack() {
	cuts := v.playback.ReplaceTrack()
	v.spawn(func(ctx context.Context) error { return v.finishInterruption(ctx, cuts) })
}

func (v *RealtimeConversation) WriteAudio(ctx context.Context, data []byte) error {
	return v.send(ctx, map[string]any{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(data)})
}

func (v *RealtimeConversation) PromptOnce(ctx context.Context, prompt *string) error {
	v.mu.Lock()

	if v.greeted {
		v.mu.Unlock()

		return nil
	}

	v.greeted = true
	v.mu.Unlock()
	instruction := v.defaultGreetingInstruction()

	if prompt != nil {
		instruction = *prompt
	}

	return v.send(ctx, map[string]any{"type": "response.create", "response": map[string]any{"instructions": instruction}})
}

func (v *RealtimeConversation) defaultGreetingInstruction() string {
	if v.language == "adaptive" {
		return "Greet the user briefly. If the user speaks another language, naturally switch to that language. The conversation is not only in English."
	}

	name := map[string]string{"ru": "Russian", "en": "English", "he": "Hebrew", "es": "Spanish", "kz": "Kazakh"}[v.language]

	if name == "" {
		name = "English"
	}

	return "Greet the user briefly in " + name + "."
}

type realtimeEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	Delta        string `json:"delta"`
	ContentIndex int    `json:"content_index"`
	Response     struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"response"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		EventID string `json:"event_id"`
	} `json:"error"`
}

func providerError(e realtimeEvent) error {
	code := e.Error.Code

	if code == "" {
		code = "realtime_error"
	}

	return &ProviderError{Provider: "openai", Code: code, Detail: e.Error.Message}
}

func (v *RealtimeConversation) dispatch(ctx context.Context, e realtimeEvent) error {
	v.mu.Lock()

	for id, at := range v.cancelEvents {
		if !at.After(time.Now()) {
			delete(v.cancelEvents, id)
		}
	}

	v.mu.Unlock()

	switch e.Type {
	case "response.created":
		v.mu.Lock()
		v.responses[e.Response.ID] = true
		v.mu.Unlock()
		state := v.playback.State()

		if state == PlaybackStateDraining || state == PlaybackStateSuspended {
			return v.cancelResponses(ctx)
		}
	case "response.output_audio.delta":
		return v.receiveAudio(ctx, e)
	case "response.output_audio.done":
		if e.ItemID == "" {
			v.playback.FinishResponse()
		} else {
			v.playback.CompleteItem(e.ItemID, e.ContentIndex)
		}

		return v.deleteItems(ctx, v.playback.TakeItemsToDelete())
	case "response.done":
		return v.responseFinished(ctx, e)
	case "input_audio_buffer.speech_started":
		return v.finishInterruption(ctx, v.playback.Interrupt())
	case "error":
		v.mu.Lock()
		_, cancel := v.cancelEvents[e.Error.EventID]
		delete(v.cancelEvents, e.Error.EventID)
		v.mu.Unlock()

		if !cancel {
			return providerError(e)
		}
	}

	return nil
}

func (v *RealtimeConversation) receiveAudio(ctx context.Context, e realtimeEvent) error {
	v.mu.Lock()
	ignore := v.cancelled[e.ResponseID] || !v.responses[e.ResponseID]

	if !ignore {
		if v.responseItems[e.ResponseID] == nil {
			v.responseItems[e.ResponseID] = map[AudioItemKey]bool{}
		}

		v.responseItems[e.ResponseID][AudioItemKey{e.ItemID, e.ContentIndex}] = true
	}

	v.mu.Unlock()

	if ignore {
		return nil
	}

	data, err := base64.StdEncoding.Strict().DecodeString(e.Delta)

	if err != nil {
		return err
	}

	before := v.playback.DroppedSamples()
	result, err := v.playback.Append(e.ItemID, e.ContentIndex, data)

	if err != nil {
		return err
	}

	switch result {
	case AppendResultOverflow:
		return v.cancelResponses(ctx)
	case AppendResultDropped:
		v.mu.Lock()
		v.droppedAudio[e.ResponseID] += v.playback.DroppedSamples() - before
		v.mu.Unlock()
	case AppendResultIgnored:
		if v.playback.State() == PlaybackStateSuspended {
			return v.finishInterruption(ctx, v.playback.Interrupt())
		}
	}

	return nil
}

func (v *RealtimeConversation) responseFinished(ctx context.Context, e realtimeEvent) error {
	v.mu.Lock()
	delete(v.responses, e.Response.ID)
	keys := v.responseItems[e.Response.ID]
	delete(v.responseItems, e.Response.ID)
	dropped := v.droppedAudio[e.Response.ID]
	delete(v.droppedAudio, e.Response.ID)
	cancelled := v.cancelled[e.Response.ID]
	delete(v.cancelled, e.Response.ID)
	v.mu.Unlock()

	for k := range keys {
		v.playback.CompleteItem(k.ItemID, k.ContentIndex)
	}

	v.playback.MarkFinishedResponse(keys)

	if err := v.deleteItems(ctx, v.playback.TakeItemsToDelete()); err != nil {
		return err
	}

	if dropped > 0 {
		v.emit(RuntimeEvent{Kind: "audio_loss", Loss: AudioLoss{Direction: "bot", DroppedSamples: int64(dropped)}})
	}

	if e.Response.Status == "failed" && !cancelled {
		return &ProviderError{Provider: "openai", Code: "response_failed", Detail: "Realtime response failed"}
	}

	return nil
}

func (v *RealtimeConversation) cancelResponses(ctx context.Context) error {
	type cancel struct{ id, event string }

	var pending []cancel
	v.mu.Lock()

	for id := range v.responses {
		if !v.cancelled[id] {
			v.cancelled[id] = true
			event := "call-cancel-" + uuid.NewString()
			v.cancelEvents[event] = time.Now().Add(30 * time.Second)
			pending = append(pending, cancel{id, event})
		}
	}

	v.mu.Unlock()

	for _, c := range pending {
		if err := v.send(ctx, map[string]any{"type": "response.cancel", "response_id": c.id, "event_id": c.event}); err != nil {
			return err
		}
	}

	return nil
}

func (v *RealtimeConversation) truncateCuts(ctx context.Context, cuts []PlaybackCut) error {
	for _, c := range cuts {
		if err := v.send(ctx, map[string]any{"type": "conversation.item.truncate", "item_id": c.ItemID, "content_index": c.ContentIndex, "audio_end_ms": c.PlayedMS}); err != nil {
			return err
		}
	}

	return nil
}

func (v *RealtimeConversation) deleteItems(ctx context.Context, items []string) error {
	for _, id := range items {
		if err := v.send(ctx, map[string]any{"type": "conversation.item.delete", "item_id": id}); err != nil {
			return err
		}
	}

	return nil
}

func (v *RealtimeConversation) finishInterruption(ctx context.Context, cuts []PlaybackCut) error {
	if err := v.cancelResponses(ctx); err != nil {
		return err
	}

	if err := v.truncateCuts(ctx, cuts); err != nil {
		return err
	}

	return v.deleteItems(ctx, v.playback.TakeItemsToDelete())
}

func (v *RealtimeConversation) watchPlayback(ctx context.Context) error {
	timer := time.NewTicker(seconds(v.options.WatchdogSeconds))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if cuts := v.playback.SuspendIfStalled(); cuts != nil {
				if err := v.finishInterruption(ctx, cuts); err != nil {
					return err
				}
			}
		}
	}
}

func (v *RealtimeConversation) Close() error {
	v.mu.Lock()
	v.closed = true
	v.cancel()
	conn := v.conn
	v.mu.Unlock()
	var err error

	if conn != nil {
		err = conn.CloseNow()
	}

	v.wg.Wait()

	if err != nil {
		return fmt.Errorf("Realtime close: %w", err)
	}

	return nil
}

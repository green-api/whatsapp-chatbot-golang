package calls

import (
	"context"
	"errors"
	sdk "github.com/green-api/whatsapp-api-client-golang-v2/voip"
	"net/http"
	"strings"
	"sync"
)

type callAPI interface {
	GetStateAsync(context.Context) (sdk.CallState, error)
	DialAsync(context.Context, string) error
	AcceptAsync(context.Context) error
	RejectAsync(context.Context) error
	HangUpAsync(context.Context) error
}

type callRTC interface {
	On(string, sdk.Callback) func()
	OpenAsync(context.Context) error
	StartAudioAsync(context.Context) error
	CloseAsync() error
}

type GreenApiCall struct {
	mu                                                                     sync.Mutex
	emit                                                                   EventSink
	api                                                                    callAPI
	rtc                                                                    callRTC
	connect                                                                func(sdk.AudioFactory) callRTC
	callID, target                                                         string
	direction                                                              CallDirection
	commandStarted, commandCompleted, remoteEnded, closing, connectionLost bool
	closeOnce                                                              sync.Once
	closeErr                                                               error
	httpClient                                                             *http.Client
}

func NewGreenApiCall(c ProviderConfig, s *CallSession, emit EventSink) *GreenApiCall {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{Transport: transport}
	api := sdk.New(sdk.Config{Host: c.APIURL, IDInstance: c.IDInstance, APITokenInstance: c.APITokenInstance, HTTPClient: client})

	return &GreenApiCall{emit: emit, api: api, connect: func(f sdk.AudioFactory) callRTC { return api.Connect(f) }, callID: s.CallID, target: s.ChatID, direction: s.Direction, httpClient: client}
}

func (a *GreenApiCall) ensureOpen() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closing {
		return errors.New("Green API call is closing")
	}

	return nil
}

func (a *GreenApiCall) Open(ctx context.Context, audio *AudioBridge) error {
	a.mu.Lock()

	if a.closing {
		a.mu.Unlock()

		return errors.New("Green API call is closing")
	}

	rtc := a.connect(audio.Create)
	a.rtc = rtc
	rtc.On("state", a.onState)
	rtc.On("error", a.onError)
	rtc.On("disconnect", a.onDisconnect)
	rtc.On("end_call", a.onEndCall)
	a.mu.Unlock()

	if err := rtc.OpenAsync(ctx); err != nil {
		return a.wrap("open", err)
	}

	return a.ensureOpen()
}

func (a *GreenApiCall) wrap(name string, err error) error {
	if err == nil {
		return nil
	}

	return wrapProviderError("greenapi", name+"_"+errorCode(err), err)
}

func (a *GreenApiCall) Dial(ctx context.Context) error {
	if err := a.ensureOpen(); err != nil {
		return err
	}

	state, err := a.api.GetStateAsync(ctx)

	if err != nil {
		return a.wrap("get_state", err)
	}

	a.mu.Lock()

	if a.closing {
		a.mu.Unlock()

		return errors.New("Green API call is closing")
	}

	if state.State != "idle" {
		a.mu.Unlock()

		return errors.New("cannot dial: instance already has a call")
	}

	a.commandStarted = true
	a.mu.Unlock()
	err = a.api.DialAsync(ctx, a.target)

	if err != nil {
		return a.wrap("dial", err)
	}

	a.mu.Lock()
	a.commandCompleted = true
	a.mu.Unlock()

	return nil
}

func (a *GreenApiCall) owns(s sdk.CallState) bool {
	if s.Info == nil {
		return false
	}

	if a.callID != "" {
		return s.Info.ID == a.callID
	}

	target := a.target

	if !strings.Contains(target, "@") {
		target += "@c.us"
	}

	return s.Info.WID == target
}

func (a *GreenApiCall) Accept(ctx context.Context) (AcceptResult, error) {
	if err := a.ensureOpen(); err != nil {
		return "", err
	}

	state, err := a.api.GetStateAsync(ctx)

	if err != nil {
		return "", a.wrap("get_state", err)
	}

	a.mu.Lock()

	if a.closing {
		a.mu.Unlock()

		return "", errors.New("Green API call is closing")
	}

	owned := a.owns(state)

	if state.State == "on-call" && owned {
		a.commandStarted = true
		a.mu.Unlock()
		a.onState(state)

		return AcceptResultAlreadyActive, nil
	}

	if state.State != "inc-call" || !owned {
		a.mu.Unlock()

		return "", &ProviderError{Provider: "greenapi", Code: "offer_gone", Detail: "Incoming offer is no longer current"}
	}

	a.commandStarted = true
	a.mu.Unlock()

	if err = a.api.AcceptAsync(ctx); err != nil {
		return "", a.wrap("accept", err)
	}

	a.mu.Lock()
	a.commandCompleted = true
	a.mu.Unlock()

	return AcceptResultCommandAcknowledged, nil
}

func (a *GreenApiCall) ReconcileAccept(ctx context.Context, cause error) error {
	state, err := a.api.GetStateAsync(ctx)

	if err != nil {
		return a.wrap("get_state", err)
	}

	a.mu.Lock()
	owned := a.owns(state)
	a.mu.Unlock()

	if owned {
		if state.State == "on-call" {
			a.onState(state)

			return nil
		}

		if state.State == "inc-call" {
			return cause
		}
	}

	if state.State == "idle" || state.Info != nil {
		return &ProviderError{Provider: "greenapi", Code: "offer_gone", Detail: "Incoming offer is no longer current"}
	}

	return cause
}

func (a *GreenApiCall) StartAudio(ctx context.Context) error {
	a.mu.Lock()
	rtc, closing := a.rtc, a.closing
	a.mu.Unlock()

	if closing || rtc == nil {
		return errors.New("Green API call is closing")
	}

	if err := rtc.StartAudioAsync(ctx); err != nil {
		return a.wrap("bridge", err)
	}

	return a.ensureOpen()
}

func (a *GreenApiCall) onState(detail any) {
	state, ok := detail.(sdk.CallState)

	if !ok {
		return
	}

	a.mu.Lock()

	if a.closing || !a.commandStarted {
		a.mu.Unlock()

		return
	}

	if state.State == "idle" {
		if a.direction == CallDirectionOutgoing && a.callID == "" && !a.commandCompleted {
			a.mu.Unlock()

			return
		}

		a.remoteEnded = true
		a.mu.Unlock()
		a.emit(RuntimeEvent{Kind: "remote_ended", Reason: state.Reason})

		return
	}

	if !a.owns(state) {
		a.mu.Unlock()

		return
	}

	a.callID = state.Info.ID
	id := a.callID
	a.mu.Unlock()

	if state.State == "on-call" {
		a.emit(RuntimeEvent{Kind: "remote_answered", CallID: id})
	}
}

func (a *GreenApiCall) onError(d any) {
	a.mu.Lock()
	skip := a.closing || a.connectionLost
	a.mu.Unlock()

	if !skip {
		a.emit(RuntimeEvent{Kind: "media_failed", Error: &ProviderError{Provider: "greenapi", Code: "rtc_error", Detail: "RTC failure"}})
	}
}

func (a *GreenApiCall) onDisconnect(d any) {
	detail, ok := d.(sdk.DisconnectDetail)

	if !ok || !detail.Permanent {
		return
	}

	a.mu.Lock()
	skip := a.closing || a.remoteEnded || a.connectionLost

	if !skip {
		a.connectionLost = true
	}

	a.mu.Unlock()

	if !skip {
		a.emit(RuntimeEvent{Kind: "media_failed", Error: &ProviderError{Provider: "greenapi", Code: "connection_lost", Detail: "Signaling permanently disconnected"}})
	}
}

func (a *GreenApiCall) onEndCall(d any) {
	detail, _ := d.(sdk.EndCallDetail)
	a.mu.Lock()
	skip := a.closing || a.remoteEnded || !a.commandStarted

	if !skip {
		if detail.Reason == "connection-lost" {
			a.connectionLost = true
		} else {
			a.remoteEnded = true
		}
	}

	a.mu.Unlock()

	if skip {
		return
	}

	if detail.Reason == "connection-lost" {
		a.emit(RuntimeEvent{Kind: "media_failed", Error: &ProviderError{Provider: "greenapi", Code: "connection_lost", Detail: "Call connection lost"}})
	} else {
		a.emit(RuntimeEvent{Kind: "remote_ended", Reason: detail.Cause})
	}
}

func (a *GreenApiCall) Finish(ctx context.Context) (FinishResult, error) {
	a.mu.Lock()
	ended := a.remoteEnded || (!a.commandStarted && a.direction == CallDirectionOutgoing)
	a.mu.Unlock()

	if ended {
		return FinishResultAlreadyEnded, nil
	}

	state, err := a.api.GetStateAsync(ctx)

	if err != nil {
		return FinishResultUnknown, a.wrap("get_state", err)
	}

	if state.State == "idle" {
		return FinishResultAlreadyEnded, nil
	}

	if state.Info == nil {
		return FinishResultUnknown, nil
	}

	a.mu.Lock()
	owned := a.owns(state)
	a.mu.Unlock()

	if !owned {
		return FinishResultNotOwned, nil
	}

	switch state.State {
	case "inc-call":
		err = a.api.RejectAsync(ctx)
	case "on-call", "out-call":
		err = a.api.HangUpAsync(ctx)
	default:
		return FinishResultUnknown, nil
	}

	if err != nil {
		return FinishResultUnknown, a.wrap("finish", err)
	}

	return FinishResultFinished, nil
}

func (a *GreenApiCall) RejectOffer(ctx context.Context) (bool, error) {
	state, err := a.api.GetStateAsync(ctx)

	if err != nil {
		return false, a.wrap("get_state", err)
	}

	a.mu.Lock()
	owned := a.owns(state)
	a.mu.Unlock()

	if state.State == "inc-call" && owned {
		err = a.api.RejectAsync(ctx)

		return err == nil, a.wrap("reject", err)
	}

	return false, nil
}

func (a *GreenApiCall) Close() error {
	a.closeOnce.Do(func() {
		a.mu.Lock()
		a.closing = true
		rtc := a.rtc
		a.mu.Unlock()

		if rtc != nil {
			a.closeErr = rtc.CloseAsync()
		}

		if a.httpClient != nil {
			a.httpClient.CloseIdleConnections()
		}
	})

	return a.closeErr
}

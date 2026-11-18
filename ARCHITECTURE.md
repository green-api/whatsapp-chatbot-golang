# whatsapp-chatbot-golang — Architecture

This document describes the **implemented structure and behavior** of the Go library. It is a factual reference, not a proposal to redesign the repository.

For agent operating rules, see [AGENTS.md](AGENTS.md). For mandatory engineering and review rules, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## 0) Scope and evidence

* This architecture map is based on the repository material reviewed while preparing the documentation: `README.md`, `go.mod`, the main bot/notification files, examples, and the `calls` implementation and tests in the supplied change snapshot.
* **Check the actual checked-out version before editing or promising a capability.** A patch, README example, or future design is not proof that a released version contains a feature.
* The `calls` package and its listed APIs refer to the reviewed implementation; they must not be advertised as present if absent from the user's installed version.
* This file describes existing components and boundaries. **New code** is governed by [CONTRIBUTING.md](CONTRIBUTING.md), even where legacy code does not yet follow that standard.

---

## 1) Project identity and responsibilities

### 1.1. Primary purpose

`whatsapp-chatbot-golang` is a **Go library for building WhatsApp chatbot applications on the public GREEN-API service**. Its reusable value includes notification receipt and dispatch, message handlers, filtering, conversation scenes/state, response helpers, and access to GREEN-API operations through the Go client.

When available in the checked-out version, `calls` adds a stateful voice-calling and real-time audio integration layer.

The library is not:

* A local implementation of the WhatsApp network or GREEN-API servers.
* A complete CRM, contact-center SaaS, or finished client-specific bot.
* A promise that every public GREEN-API endpoint has a dedicated chatbot abstraction.
* An unlimited concurrent-call scheduler or an automatically provider-neutral voice platform.

### 1.2. Source and dependency boundaries

The reviewed `go.mod` declares:

```text
module github.com/green-api/whatsapp-chatbot-golang

go 1.24.0
toolchain go1.24.2
```

It includes `github.com/green-api/whatsapp-api-client-golang-v2` and, for the reviewed calling implementation, WebSocket/WebRTC/Opus-related dependencies. **Treat the currently checked-out `go.mod` as authoritative**, including for the actual toolchain and versions; these values are not promises for all releases.

Public GREEN-API references:

* Product: https://green-api.com/
* Public API documentation: https://green-api.com/en/docs/api/
* Developer console: https://console.greenapi.com/

Do not confuse API documentation with a guarantee that the local wrapper exposes every operation in the same form.

---

## 2) Package layout and code placement

The paths below are grounded in the reviewed source snapshot; verify them in the current checkout.

| Area | Files / directories | Existing responsibility |
| --- | --- | --- |
| Module | `go.mod`, `go.sum` | Import identity, Go version, dependencies |
| Root bot | `bot.go` | `Bot`, `NewBot`, notification receive loop, dispatch and acknowledgment |
| Notification data | `notification.go` | `Notification`, sender/chat accessors, notification context |
| Message helpers | `service.go`, legacy-spelled `sevice.go` where present | Sending/responding helpers associated with notification/bot behavior |
| Getting started | `README.md`, `docs/README_RU.md` | Setup, examples and public-facing guidance |
| Bot examples | `examples/base`, `examples/echo`, `examples/filter`, `examples/event`, `examples/state`, `examples/full` | Application-level notification/handler/scene patterns |
| Call models | `calls/models.go` | `CallSession`, states, events, deadlines, options, errors, result types |
| Call service | `calls/service.go` | `WhatsAppCallService`, constructors, admission, stopping, execution |
| Call transitions | `calls/state_machine.go` | Transition rules and `CallStateMachine.Apply` |
| Call runner | `calls/call_runner.go` | eventQueue, operation tracking, deadlines, execution, resource cleanup; interfaces and dependency factories |
| Calling adapter | `calls/greenapi.go` | Provider integration with GREEN-API's calling transport/SDK |
| Voice adapter | `calls/openai.go` | Default `RealtimeConversation` implementation using an external real-time voice provider |
| Audio bridge | `calls/audio.go` | `AudioSource`/`AudioSink`, buffers, tracks, recording tap, media bridge |
| Resampling | `calls/audio_resampler.go`, `calls/audio_tone.go` | PCM sample-rate conversion and optional voice tone processing |
| Playback | `calls/playback_state_machine.go` | Playback state, buffering, interruption/drain behavior |
| Recording | `calls/recorder.go` | Capture and export of call tracks; recording lifecycle |
| Call examples | `examples/calls/incoming`, `examples/calls/outgoing` | Example call-session integration; may involve real external calls |
| Tests | `calls/*_test.go`, `test/calls/*_test.go` | Behavior, compatibility, cancellation, timing and resource-cleanup checks |

`sevice.go` is the spelling observed in the source snapshot. Do not infer that a corrected file exists or rename it solely for style; inspect references and compatibility first.

---

## 3) Messaging and bot architecture

### 3.1. Public-facing bot entry

The reviewed root package is declared as `package whatsapp_chatbot_golang` in source; the `chatbot` name seen in usage samples is an **import alias**, not necessarily the declared Go package name. Verify package declarations and paths before writing imports.

`NewBot(IDInstance, APITokenInstance)` creates a `Bot` configured with instance credentials and GREEN-API client behavior. The reviewed `Bot` embeds `greenapi.GreenAPI` from `whatsapp-api-client-golang-v2`, and also contains `StateManager` and `Publisher` behavior.

Common public usage relies on handlers (including `IncomingMessageHandler`), `Notification`, filters and response methods, and `StartReceivingNotifications`. Check current exact signatures in source or tests before providing copy/paste code.

### 3.2. Incoming notification flow

```text
GREEN-API notification queue
  -> Bot.StartReceivingNotifications / receiving client
  -> ReceiveNotification
  -> parse response envelope and body
  -> construct Notification
  -> dispatch matching handler / scene / state behavior
  -> optional outgoing GREEN-API calls
  -> DeleteNotification(receiptId)
```

In the reviewed `bot.go`, the receive loop distinguishes an empty response from an error, parses JSON from the returned body, constructs a `Notification`, dispatches it through `startCurrentScene`, and then invokes `DeleteNotification`.

**Important behavior:** notification acknowledgment is a distinct operation after handler/scene processing. Changing parsing, error handling, receipt handling, or acknowledgment order may result in repeated or lost notifications. Preserve the observed behavior unless the change explicitly addresses it.

**Observed implementation risks, not intentional contracts:** the reviewed changes include unchecked map type assertions and raw notification logging in the receive path. Do not turn those implementation details into endorsed design patterns; when touching this area, prioritize safe decoding, redaction, and behavior-preserving tests.

### 3.3. Application conversation state

Handlers, filters, Scenes and `StateManager` organize application-level behavior. These are the preferred integration surfaces for a customer's conversation flow; a customer-specific state machine normally belongs to the customer's application, not the shared library.

Do not assume that the bot's conversation state and the call-session state machine are the same system. They serve distinct responsibilities.

### 3.4. Outgoing messages

Response helpers in the reviewed root files use GREEN-API Go-client sending methods. Some return generic maps and send errors through `ErrorChannel`. Treat current return values and error propagation as **publicly observable** even when a different design would be cleaner.

Calling a send method is not proof of final delivery or read status. Refer to GREEN-API's current documentation for the semantics of the relevant endpoint or notification.

---

## 4) Calling subsystem (when present)

### 4.1. Call execution overview

```text
Application creates CallSession and transition callback
  -> WhatsAppCallService.Execute
  -> admission gate / callRunner
  -> runtime eventQueue + deadlines + state transitions
  -> CallProvider (GREEN-API integration)
  -> VoiceConversation (default real-time voice integration)
  -> AudioBridge / PlaybackController / optional CallRecorder
  -> terminal state, result and staged cleanup
```

This is a **stateful, multi-component runtime**, not a single synchronous API call. Its behavior is distributed across session models, transition rules, event consumption, provider callbacks, timeouts, media handling, and cleanup.

### 4.2. Service and concurrency boundary

`WhatsAppCallService` in the reviewed `calls/service.go` is documented as **single-instance**. Its admission gate serializes `Execute`; `RequestStop` closes admission and notifies the active runner. `Healthy` and `NotifyIncomingEnded` expose limited lifecycle interaction.

Do not infer unlimited concurrency, multi-tenant scheduling, or automatic queue management from this API. A multi-call application must arrange service ownership and scheduling explicitly.

### 4.3. Call session and authoritative transitions

`CallSession` stores call direction, current state, identity, timestamps, reasons and other runtime information. `CallStateMachine` defines allowed `(state, event)` transitions through rules and resolvers; `Apply` changes a session and returns a `StateTransition`.

Representative states in the reviewed model include `triggered`, `queued`, `dialing`, `ringing`, `connecting`, `in_call`, `ending`, and terminal states such as `remote_ended`, `ring_timeout`, `talk_timeout`, `cancelled`, `rejected`, `failed`.

`TransitionCallback` applies transitions synchronously on the execution goroutine. The runner uses the resulting session state, not the built-in rule table, to close event admission. A successful `NotifyIncomingEnded` means an event was queued; events queued during a callback are consumed only if the resulting state allows execution to continue. Initial validation errors leave session state unchanged.

Transition rules differ where incoming and outgoing flows have different semantics. An invalid or unmatched event does not automatically become a valid transition. Do not mutate `CallSession.State` in unrelated modules to bypass `CallStateMachine`.

### 4.4. Event loop, operations and deadlines

The runner, event queue, deadlines, operation tracking, resource owner and transition rules are package-private implementation details. Applications use sessions, the state machine and `WhatsAppCallService`; provider integrations use the exported interfaces below.

`callRunner` coordinates asynchronous command completion/failure, provider events, stop requests and deadline expiry. `eventQueue.Receive` handles events relative to observation time; `deadlines` tracks timers. `operations` starts cancellable work and reports results back as runtime events.

Ordering matters: remote answer, command completion, cancellation, remote end, deadline expiry and cleanup can race. Verify both success and error timelines, not just the happy path.

### 4.5. Provider boundaries and extension points

The public provider extension points in `calls/call_runner.go` are:

* `CallProvider` — calling operations such as `Open`, `Dial`, `Accept`, `StartAudio`, `Finish`, `RejectOffer`, and `Close`.
* `VoiceConversation` — an `AudioSource` and `AudioSink` with `Start`, `PromptOnce` and `Close`.
* `Dependencies` — `NewAPI` and `NewVoice` factories.

`NewWhatsAppCallServiceWithDependencies` accepts injected factories. This is a real potential boundary for changing the voice provider without rewriting call orchestration.

**Observed constructor limitation:** in the reviewed implementation, `NewWhatsAppCallServiceWithDependencies` validates an `OpenAIAPIKey` even when a custom `NewVoice` is injected. Therefore an interface alone does not prove provider replacement is fully configuration-independent. Verify constructor, factory, audio, and cleanup paths as one contract before promising White Label compatibility.

Common error codes are `validation_error`, `timeout` and `runtime_error`. Provider codes retain their own names; cleanup codes include the stage and underlying code.

`ProviderError` preserves the original provider error in `Cause` via `Unwrap`, while `Detail` contains explicit provider diagnostics and `Error` exposes only provider identity and code.

### 4.6. Incoming and outgoing calls

`NewCallSession` creates an outgoing-style session; `NewIncomingCallSession` also requires an incoming call ID and deadline. Application code provides a transition callback and runs `Execute` after the applicable queue/dequeue transitions.

The supplied incoming example selects a call offer ID provided from outside the example; it does **not** prove automatic discovery/routing of all incoming calls. Examples making real calls require deliberate execution and configured services.

---

## 5) Audio, playback and recording

### 5.1. PCM contract

The reviewed internal voice/recording path uses **signed PCM16, mono, 24 kHz**, with **20 ms frames** (480 samples / 960 bytes per frame). The RTC SDK path uses 48 kHz frames where relevant; `pcmRateConverter` performs conversion at the boundary.

Treat sample format, frame duration, timestamp/PTS, buffer capacity, overflow policy and filter continuity as coordinated media contracts. Changing one without checking all consumers can cause timing drift, malformed frames, loss or artifacts.

### 5.2. Media components

* `AudioSource.ReadFrame` supplies playback frames; `AudioSink.WriteAudio` receives caller audio.
* `InputAudioBuffer` bounds queued caller audio, with configured policies such as dropping oldest frames or reporting overflow.
* `BotTrack` produces outgoing frames; `CallerSink` receives incoming tracks.
* `AudioBridge` connects both directions and recording taps; replacing a track resets related media state.
* `PlaybackController` manages response audio, pending frames, interruptions and drain behavior using a separate playback FSM.
* Audio-loss and media-failure events are surfaced to the calling runner.

### 5.3. Recording ownership

`CallRecorder` and its capture/export helpers manage audio recording, eventual file output and close/discard behavior. `TakeReadyOrDiscard` transfers a ready file or discards a pending export. The internal PCM format is fixed by the exported PCM constants; recording options configure the encoder and buffering. `CallExecutionResult` exposes recording output ownership through methods such as `RecordingPath`, `TakeRecording` and `Close`.

The reviewed examples use FFmpeg for optional MP3 export when available; this is an **external executable prerequisite**, not functionality bundled with the Go runtime.

`CallRecorder.Finish` reports capture and encoder failures through its error result. `RecordingError` preserves the original cause through `Unwrap` and provides bounded encoder diagnostics separately from its safe error text. Recording failures remain optional for call execution; they do not alone change a completed conversation into a failed call. `Execute` returns these failures separately through `result.RecordingError()`, preserving their causes for `errors.Is/As`. This is a snapshot of the failure known when execution returned, including an export timeout; late completion does not change it. A nil recording error and empty path are valid when recording is disabled, has no audio, or is discarded by retention policy. `TakeRecording` and `Close` preserve the diagnostic.

```go
result, err := service.Execute(ctx, session, fsm.Apply)

if err != nil {
    return err
}

defer result.Close()

if recordingErr := result.RecordingError(); recordingErr != nil {
    // Report the recording failure independently of the conversation outcome.
}
```

Resource cleanup is staged and may be bounded by timeouts. Do not assume `Close` is instantaneous or that every remote finish is confirmed merely because local closure returned.

---

## 6) External dependencies and configuration

* GREEN-API instance ID and token are required for the corresponding service operations; account authorization and notification settings govern whether messages/notifications work.
* The default real-time voice integration in the reviewed `calls` package uses external OpenAI Realtime endpoints, model/voice configuration and credentials.
* Calling transport, real-time audio transport and the external voice conversation are separate concerns. Replacing one does not automatically replace the others.
* Network and service errors, timeout behavior, retries, provider availability and account policy are externally controlled and must be represented accurately.
* Secrets, message bodies, caller identifiers, raw events and recordings are sensitive. Prefer redacted structured diagnostics.

Refer to the current official API documentation for exact provider-side behavior. Do not invent unsupported API methods or make guarantees about message delivery or call quality.

---

## 7) Public and implicit contracts

Compatibility-sensitive surfaces include:

1. `go.mod` import path, declared Go/toolchain version, supported dependencies.
2. Exported Go types, interfaces, constructors, methods, enum-like constants and struct fields.
3. Bot/notification data shape, handler dispatch, receipt acknowledgment and error channels.
4. Scene/state behavior and customer-facing helper methods.
5. Calling service admission, session states, transition events, deadlines and error classifications.
6. Interface/factory signatures, validation defaults and third-party provider boundaries.
7. PCM format, frame timing, conversion, buffering, event ordering and cleanup.
8. Recording ownership, export behavior, returned results and file lifetimes.
9. Working examples, documentation imports and installation instructions relied upon by users.

A documented interface is not necessarily the only contract; external consumers may depend on observed return values, error shape, timing or default behavior. Preserve compatibility by default. Breaking changes require an explicit decision, migration notes and tests.

---

## 8) White Label architecture boundaries

Default locations for customer-specific changes:

* **Application layer:** branding, messages, language, workflows, prompts, CRM integration and business policy.
* **Public configuration:** instance credentials, supported runtime options and environment-specific settings.
* **Existing extension contracts:** injected `VoiceConversation` / `CallProvider` implementations and other real callbacks/interfaces.
* **Shared library:** only a small reusable extension when the intended customization cannot be achieved externally.
* **Fork / module-path migration:** a deliberate ownership, compatibility and distribution decision, not a normal setup step.

Changing application branding is not the same as changing the Go module identity. Inspect the actual `LICENSE` before redistributing modifications or changing notices. The project uses the [MIT License](LICENSE), which permits modification and redistribution subject to its terms, including preservation of the copyright and permission notices.

---

## 9) Architecture investigation map

| When touching… | Inspect at minimum… | Preserve / verify… |
| --- | --- | --- |
| Bot receiving | `bot.go`, `notification.go`, receive SDK, handler tests | Parsing, dispatch, receipt IDs, acknowledgement |
| Scenes and replies | Handler/scene files, `service.go` or `sevice.go`, examples | Filters, state and error behavior, public API |
| Public API or module | `go.mod`, exported symbols, README, examples and consumers | Imports, defaults, signatures, migrations |
| Call states | `calls/models.go`, `state_machine.go`, `call_runner.go` | Allowed transitions, end reasons, event ordering |
| Call lifecycle | `calls/service.go`, `call_runner.go`, `greenapi.go` | Admission, deadlines, cancellation, cleanup |
| Voice provider | `call_runner.go` interfaces, service constructor, `openai.go` | Factory validation, audio and lifecycle contract |
| Audio pipeline | `audio.go`, resampler, tone, playback, media tests | PCM rate, timestamps, overflow, loss events |
| Recording | `recorder.go`, `models.go`, call runner, tests | Result ownership, FFmpeg failures, close/discard |
| White Label | Public options, interfaces, constructors, examples, `LICENSE` | Upgradeability, compatibility, distribution constraints |

The **required engineering process and review standards** are in [CONTRIBUTING.md](CONTRIBUTING.md). Do not change the architecture merely because a new pattern seems aesthetically preferable; show the actual responsibility or compatibility problem first.

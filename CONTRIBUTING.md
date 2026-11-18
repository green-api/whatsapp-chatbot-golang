# whatsapp-chatbot-golang — Contributing

Instructions for maintainers, external contributors, and AI coding agents implementing or modifying this repository.

This document is the **mandatory engineering and code review policy for new work**. It defines the quality expected of a contribution, including rules that may be stricter than the patterns found in legacy code.

For agent workflow and product explanation, see [AGENTS.md](AGENTS.md). For the implementation map and existing runtime contracts, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 0) Engineering objective

The code should be **correct, explicit, testable, understandable, and boring in a good way**. A developer unfamiliar with the project should normally be able to understand any single function's purpose, inputs, outputs and side effects in **five minutes or less**.

Requirements:

* Correctness, public compatibility, safe lifecycle and maintainability take priority over token usage and code brevity.
* Reuse existing code and standard-library facilities before introducing a dependency or a new abstraction.
* Keep the smallest **complete** safe change, including relevant tests and documentation.
* Preserve public/implicit contracts by default; a cleaner-looking design is not enough reason to break downstream users.
* **New code must comply with this policy. Modified code must move toward compliance within the affected scope.** Do not rewrite unrelated legacy code only to satisfy the style policy.
* When this document and existing code differ, use the existing code to understand actual behavior and this document to choose the style of new implementation. Do not silently change behavior.

---

## 1) Contribution workflow

### 1.1. Before writing code

1. **State the outcome:** what behavior must change, what must remain unchanged, and what proves completion.
2. **Trace the affected path:** find the public entry point, implementation, direct callers, errors, resource owner and relevant tests. Use [ARCHITECTURE.md](ARCHITECTURE.md) for the subsystem map.
3. **Identify contracts:** public Go API, notification acknowledgments, state transitions, concurrency, network behavior, provider validation, audio formats, or cleanup if applicable.
4. **Reuse first:** determine whether the change can use existing configuration, an existing application API, an interface/factory or a small local helper.
5. **Choose the smallest complete design:** avoid unnecessary frameworks, public APIs and dependencies.
6. **Check risk:** if a task changes public behavior, lifecycle, concurrency, media or security, plan failure-path and compatibility tests before implementation.

### 1.2. While implementing

* Keep functions focused, small and explicitly named.
* Make state and ownership visible; errors must not be silently discarded.
* Keep implementation details behind appropriate dependency boundaries.
* Preserve unrelated formatting, APIs, imports and behavior.
* When new reusable behavior is added, update the public example or package documentation that should teach it.
* Never commit credentials or production payloads.

### 1.3. Before reporting completion

* Run focused tests for affected behavior; run wider checks when risk warrants it.
* Inspect the final diff for unrelated changes and code review violations.
* Explain any intentional compatibility impact, migration and remaining uncertainty.
* Report **exact commands executed** and results; never treat unrun tests as passed.

---

## 2) Functions: size, responsibility and contracts

### 2.1. Strict size limit

**New and materially modified functions MUST contain no more than 50 nonblank, noncomment lines in the function body.** Aim for **30 lines or fewer** when reasonable. This is a corporate code-review requirement, not a general preference.

* If a new function would exceed 50 lines, divide it along real responsibilities before submitting.
* Avoid deep nesting, even when the function is under 50 lines.
* Extract helpers that name meaningful operations; do not create a sequence of trivial forwarding wrappers merely to pass the limit.
* Avoid unnamed local functions whose large bodies only hide complexity from the line count.
* For a pre-existing oversized function, confine refactoring to the changed responsibility and tests. Do not expand it further. A broader cleanup requires its own task.
* A rare exception requires an explicit reviewer-approved reason, not an agent's unilateral judgment.

If a configured linter counts lines differently, CI's configured measurable rule is the enforcement baseline; keep the underlying goal of small, readable function bodies.

### 2.2. Single responsibility

Every function must have a **specific reason to exist** that can be expressed in one sentence.

A good function has:

1. Clear inputs with meaningful names and types.
2. A clear output or observable effect.
3. One primary responsibility.
4. Defined behavior for invalid input, cancellation and errors.
5. No hidden dependence on unrelated global state.

For stateful functions, the contract must also make relevant state changes and resource ownership understandable.

### 2.3. Example — avoid a blob function

**Avoid:** a large handler that validates input, builds provider requests, changes session state, sends data and logs errors within one body.

```go
// BAD: one method mixes application policy, provider details,
// state mutation and error handling.
func (s *Service) Process(data map[string]any) error {
    // parse and validate input...
    // choose provider settings...
    // perform network call...
    // mutate session state...
    // write audit record...
    // update retry counters...
    return nil
}
```

**Prefer:** a readable orchestrator with named steps, each with its own contract. The following is a synthetic pattern, not an API already provided by this repository:

```go
func (s *NotificationService) ProcessIncomingMessage(
    ctx context.Context,
    notification Notification,
) error {
    message, err := parseIncomingMessage(notification)
    if err != nil {
        return fmt.Errorf("parse incoming message: %w", err)
    }

    if err := validateIncomingMessage(message); err != nil {
        return fmt.Errorf("validate incoming message: %w", err)
    }

    if err := s.handler.HandleMessage(ctx, message); err != nil {
        return fmt.Errorf("handle incoming message: %w", err)
    }

    return nil
}
```

Parsing, validation and handling are separate because they are independently understandable operations. **Do not assume this sample defines the current queue-acknowledgment policy**; the real notification flow must still be traced before editing it.

---

## 3) Names: simple English and literal responsibilities

### 3.1. Naming policy

Use **ordinary, simple English**. Prefer names that are slightly longer but unmistakable over short, clever or abstract names. A teammate who is not fluent in English should still understand the action.

Requirements:

* Nouns for values and domain entities; verbs for operations.
* Boolean names describe a verifiable condition: `hasAllRequiredFields`, `isSessionExpired`.
* Validation names state what is checked: `validateProviderCredentials`, not `ensureRules`.
* Mutating or I/O methods must not look like pure queries.
* Use standard Go terms such as `Close`, `Read`, `Write`, `Validate`, `New` when they have conventional meanings.
* Avoid unexplained abbreviations, literary names and vague infrastructure names such as `MagicManager`, `FlowOrchestrator`, `ProcessData`, `DoWork`.
* Consider the package name when naming symbols. In Go, `callrules.Validate(rule)` may be clearer than `callrules.CallRulesServiceValidateRule(rule)`.
* Avoid broad style-driven renames of existing exported APIs; source compatibility outweighs naming preference in unrelated work.

### 3.2. Concrete examples

| Weak / ambiguous | Preferred when behavior matches |
| --- | --- |
| `ensureRules(value)` | `hasAllRequiredFields(rule)` |
| `Process(data)` | `ProcessIncomingNotification(ctx, notification)` |
| `fixState()` | `ApplyCallStateTransition(session, event)` |
| `doCleanup()` | `CloseCallResources()` or conventional `Close()` on the owner |
| `check(x)` | `isSessionExpired(session, now)` |
| `executeFlow()` | `ReceiveAndDispatchNotifications(ctx)` |

These names illustrate the standard; do not create duplicate functions solely to match the table. In particular, a `bool` query and an `error`-returning validator are different contracts.

### 3.3. Comments

Comments should normally explain **why**, not translate obvious Go syntax into prose. Document non-obvious timing, compatibility rules, resource ownership, lifecycle and public contracts. Exported declarations should have idiomatic Go doc comments where useful.

Good:

```go
// NOTE: A receipt is acknowledged after handler dispatch.
// Changing the order can lose notifications on handler failure.
```

Bad:

```go
count++ // increment count by one
```

---

## 4) Dependency direction, contracts and testability

### 4.1. Required design rule

High-level application and orchestration code should depend on **small contracts for real external or variable behavior**, not concrete HTTP/SDK/database implementations.

* Define interfaces close to the consumers that need them.
* Keep each interface as small as the consumer requires.
* Inject dependencies at explicit construction boundaries.
* Put provider-specific parsing, network calls and protocol details in lower-level adapters.
* Use concrete types for internal local helpers with no actual variation or test seam.
* Do not manufacture interfaces for every struct or speculative future provider.
* Ensure the complete construction path supports injection; an exposed interface is not enough if constructor validation still hardcodes a specific provider.

### 4.2. Example — consumer-owned interface

**Avoid:** a high-level workflow constructing or importing a concrete external client and mixing network semantics with business decisions.

```go
// BAD: external transport details leak into the workflow.
func (s *WelcomeService) SendWelcome(ctx context.Context, chatID string) error {
    client := NewConcreteTransport(s.token)
    return client.SendProviderPayload(ctx, map[string]any{
        "chatId": chatID,
        "message": "Welcome!",
    })
}
```

**Prefer:** a contract defined by the consumer's actual need. The following is an independent, complete illustrative Go example (not a current library API):

```go
package example

import (
    "context"
    "errors"
    "fmt"
)

type TextSender interface {
    SendText(ctx context.Context, chatID, text string) error
}

type WelcomeService struct {
    sender TextSender
}

func NewWelcomeService(sender TextSender) (*WelcomeService, error) {
    if sender == nil {
        return nil, errors.New("text sender is required")
    }
    return &WelcomeService{sender: sender}, nil
}

func (s *WelcomeService) SendWelcome(ctx context.Context, chatID string) error {
    if chatID == "" {
        return errors.New("chat ID is required")
    }
    if err := s.sender.SendText(ctx, chatID, "Welcome!"); err != nil {
        return fmt.Errorf("send welcome message: %w", err)
    }
    return nil
}
```

A test can inject a fake without connecting to an actual provider:

```go
package example

import "context"

type FakeTextSender struct {
    ChatID string
    Text   string
}

func (f *FakeTextSender) SendText(
    _ context.Context, chatID, text string,
) error {
    f.ChatID = chatID
    f.Text = text
    return nil
}
```

This is dependency inversion **at a real boundary**. It is not permission to wrap every standard library function in a new interface.

---

## 5) Explicit state machines where the domain requires them

### 5.1. When to use an FSM

Use a finite-state machine when correctness depends on a restricted set of **states, events and allowed transitions**, for example call lifecycle, job lifecycle, payment status or a multi-step workflow with invalid transitions.

Requirements:

* Name states and events explicitly.
* Put authoritative transition rules in one place.
* Give one component ownership of state mutation.
* Distinguish invalid, ignored and idempotent/repeated events deliberately.
* Keep transition decisions separate from provider I/O wherever practical.
* Test permitted and forbidden transitions, repeat events, terminal behavior, and error paths.
* Do not introduce a new FSM for a simple boolean or local variable with no transition constraints.

**Existing example:** `calls/state_machine.go` has `CallStateMachine.Apply`, `TransitionRule` and `callRules`. It is a domain-specific implementation, not a requirement to use the exact same type names for every new workflow.

### 5.2. Example — centralized pure transition function

```go
package example

import "fmt"

type JobState string
type JobEvent string

const (
    JobQueued  JobState = "queued"
    JobActive  JobState = "active"
    JobDone    JobState = "done"
    StartJob   JobEvent = "start"
    FinishJob  JobEvent = "finish"
)

type transitionKey struct {
    state JobState
    event JobEvent
}

var allowedTransitions = map[transitionKey]JobState{
    {JobQueued, StartJob}: JobActive,
    {JobActive, FinishJob}: JobDone,
    {JobDone, FinishJob}: JobDone, // explicitly idempotent
}

func NextJobState(state JobState, event JobEvent) (JobState, error) {
    next, ok := allowedTransitions[transitionKey{state, event}]
    if !ok {
        return state, fmt.Errorf(
            "invalid transition: state=%s event=%s", state, event,
        )
    }
    return next, nil
}
```

A single owner should call `NextJobState` and commit the result; adapters must not independently set `JobState` to arbitrary strings. Add tests for accepted/invalid transitions. This example is synthetic and independent of the WhatsApp call FSM.

### 5.3. Common failure modes

* Handlers independently mutating the same state without a transition owner.
* A provider response being interpreted as successful completion without observing the required terminal event.
* Multiple state enum/transition implementations gradually diverging.
* Terminal states being silently reopened by late callbacks.
* Hidden timeouts or retries changing transitions without explicit events.

---

## 6) Resource ownership, cancellation and concurrency

### 6.1. Ownership rule

Every resource must have a clear owner: open files, sockets, buffers, media streams, goroutines, channels, timers and provider sessions.

The owner must define who starts it, when it ends, how it is canceled and how errors are observed. Higher-level code should call a clear method such as `Close()` without knowing each lower-level cleanup step.

Requirements:

* Use standard `Close` conventions and document whether a method is idempotent.
* Apply `context.Context` to cancelable work and respect returned errors.
* Avoid holding locks during slow I/O or callbacks unless proven safe and necessary.
* Avoid unmanaged goroutines and channels with no shutdown protocol.
* Keep temporary and permanent failures distinguishable where behavior relies on it.
* Preserve established shutdown, deadline, retry, resource ownership and recording semantics when modifying `calls`.
* Do not silently discard errors from resource closure; combine or report them with context where appropriate.

### 6.2. Example — one owner, one close contract

```go
package example

import (
    "errors"
    "io"
    "sync"
)

type SessionResources struct {
    voice    io.Closer
    provider io.Closer
    once     sync.Once
    closeErr error
}

func (r *SessionResources) Close() error {
    r.once.Do(func() {
        if r.voice != nil {
            r.closeErr = errors.Join(r.closeErr, r.voice.Close())
        }
        if r.provider != nil {
            r.closeErr = errors.Join(r.closeErr, r.provider.Close())
        }
    })
    return r.closeErr
}
```

This example defines **idempotent local closure** for two resources. It does not replace the actual call runner's staged remote-finish logic, deadlines, recording decisions or asynchronous cleanup handling.

**Avoid:** top-level code calling `StopVoice`, `DisconnectProvider`, `DrainBuffer`, and `CloseRecorder` in a hardcoded sequence when the resource owner should perform those operations.

---

## 7) Error handling, boundaries and observability

* Validate input at real trust boundaries (public API, external payloads, provider callbacks, configuration), not redundantly at every internal line.
* Check type assertions and decoded fields before use; avoid panic-prone assumptions about external responses.
* Wrap propagated errors with concrete operation context, using `%w` when callers need `errors.Is` / `errors.As` behavior.
* Never transform temporary provider failure into apparent success merely to simplify control flow.
* Use explicit timeouts where network or shutdown behavior requires them; do not invent fallback results that hide an incomplete call.
* Prefer structured, redacted diagnostic fields over raw notification/voice/call payloads.
* Do not print GREEN-API tokens, model API keys, phone numbers, private audio, raw messages or other sensitive customer content in tests, logs or examples.
* When errors trigger side effects (acknowledgment, retry, cleanup, state transition), test those effects, not only the returned error string.

---

## 8) White Label contribution rules

**A White Label adaptation should primarily live outside the shared library.** Choose the lowest sufficient level:

1. Existing settings or configuration.
2. Customer application using public APIs, handlers, Scenes and client-owned logic.
3. Existing adapter/interface/factory injection.
4. A small generic extension point with a documented contract and tests.
5. A targeted shared-core change with compatibility reasoning.
6. Fork or module-path migration only by explicit decision.

Before changing the library for a client, check the constructor path, interface implementation, provider lifecycle and example integration **end to end**. For voice integration, consider `VoiceConversation`, `Dependencies.NewVoice`, and the constructor's current validation; do not promise custom provider support solely from the presence of an interface.

Do not bake customer names, credentials, brand claims or business workflows into generic code. Do not assume that “White Label” grants rights to remove notices, change the library's identity or redistribute modified sources; inspect the current `LICENSE` and use appropriate licensing review.

---

## 9) API compatibility and dependencies

* Preserve exported Go names, signatures, struct fields, constants, constructor behavior and default values unless change is intentionally approved.
* Treat Go module paths, import aliases, docs and runnable examples as developer-facing compatibility surfaces.
* Avoid a new dependency when the standard library or an existing package solves the problem sufficiently.
* Do not replace existing state/transport behavior with an invented protocol or abstraction.
* A new public extension point must have a narrow contract, one demonstrated use case, a useful example and tests exercising an alternative implementation where relevant.
* A breaking change requires explicit migration guidance and tests; do not hide it in a cosmetic cleanup.

---

## 10) Tests, tools and review gates

### 10.1. Required checks by change type

| Changed area | Essential verification |
| --- | --- |
| Pure helpers / validation | Correct inputs, boundaries, errors, names, function size |
| Public Go API | Existing callers compile; old defaults and error semantics preserved |
| Notifications / handlers | Parse failures, routing, state effects, receipt acknowledgment |
| Call state | Valid/invalid/repeated/terminal transitions, incoming/outgoing distinctions |
| Concurrency / cleanup | Cancellation, service admission, races, late callbacks, partial close failures |
| Audio / recorder | PCM frame size/timestamps, resampling, buffer overflow, recording ownership |
| White Label extension | Alternative construction, execution, error and shutdown through the injected implementation |
| Documentation / examples | Real imports and methods, exact prerequisites, no live secrets |

Useful commands when available in the checkout:

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
```

Where the repository enables `golangci-lint`, use its configured `funlen`/complexity checks for the 50-line limit. Do not invent a local `golangci-lint` configuration or pretend that an unconfigured tool ran. A future CI gate should enforce the function-size policy for changed code while legacy issues are addressed separately.

**Do not run live GREEN-API sends/calls or billable external provider integration tests without the necessary environment and explicit test intent.** Offline tests are not evidence that a live WhatsApp call succeeded.

### 10.2. Code review rejection criteria

A change should be rejected or revised when it:

* Adds or materially expands a function beyond the 50-line limit without an explicitly approved exception.
* Uses vague names, hidden side effects, unrelated responsibilities or deeply nested logic.
* Couples high-level workflow to concrete provider behavior where a real narrow contract is needed.
* Invents interfaces/abstractions with no consumer or current requirement.
* Scatters FSM transitions or creates multiple state owners.
* Leaves resource lifecycle, goroutines, cancellation or error handling unclear.
* Changes public APIs, notification acknowledgment or runtime media/state contracts without impact review.
* Introduces client-specific branding or business rules into the shared core unnecessarily.
* Omits failure-path tests for changed lifecycle/contract behavior.
* Claims tests, external provider checks or compatibility guarantees without evidence.

### 10.3. Expected change summary

```text
Changed:
- [Concrete behavior and files]

Preserved:
- [Public APIs and relevant runtime contracts]

Corporate standard:
- [Function-size compliance; dependency / FSM / ownership decisions]

Verified:
- [Commands run and precise results]

Not verified / risks:
- [Environment limits, provider paths, migration issues]
```

---

## 11) Do Not

* ❌ Don't submit new or materially modified functions exceeding 50 nonblank, noncomment lines without explicit reviewer approval.
* ❌ Don't satisfy function-size rules with meaningless wrappers or hidden giant closures.
* ❌ Don't use vague names or mix independent responsibilities in one function.
* ❌ Don't create speculative interfaces, dependency layers or frameworks without a real use case.
* ❌ Don't let high-level policy code depend on concrete provider internals when a narrow real boundary is needed.
* ❌ Don't duplicate FSM transition rules or mutate centrally owned state through unrelated modules.
* ❌ Don't create resources, goroutines or timers without an owner and shutdown mechanism.
* ❌ Don't swallow errors, leak secrets or treat mocked tests as live-provider validation.
* ❌ Don't break public APIs or old behavior for style alone.
* ❌ Don't turn customer-specific White Label needs into unnecessary shared-core coupling.
* ❌ Don't broaden the diff through unrelated rewrites or dependency upgrades.

---

## 12) Pre-change checklist

* [ ] The outcome, changed boundary and acceptance criteria are clear.
* [ ] The relevant architecture, public entry points, consumers and tests were inspected.
* [ ] Existing configuration, public APIs and extension points were considered first.
* [ ] New/modified function bodies meet the 50-line rule; no meaningless wrappers were added.
* [ ] Each function has one responsibility, understandable inputs/outputs and a clear reason to exist.
* [ ] Names use simple, concrete words and established Go conventions.
* [ ] Real dependency boundaries use small consumer-oriented contracts when appropriate.
* [ ] Stateful workflows have one owner and explicit transitions where needed.
* [ ] Resources, contexts, goroutines and cleanup have clear ownership.
* [ ] Error and failure paths are observable and tested.
* [ ] Public compatibility, external dependencies and White Label upgrade paths were respected.
* [ ] Relevant tests and review tools were actually run and their results recorded.
* [ ] Documentation/examples and any migration notes were updated where relevant.
* [ ] Remaining limitations and risks were reported honestly.

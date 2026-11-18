// Accept one application-selected incoming offer. Obtain its ID from notifications.
package main

import (
	"context"
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	config := calls.ProviderConfig{APIURL: os.Getenv("GREEN_API_HOST"), IDInstance: os.Getenv("GREEN_API_ID"), APITokenInstance: os.Getenv("GREEN_API_TOKEN"), OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"), RealtimeModel: os.Getenv("OPENAI_REALTIME_MODEL"), RealtimeVoice: os.Getenv("OPENAI_REALTIME_VOICE")}

	if config.RealtimeModel == "" || config.RealtimeVoice == "" {
		log.Fatal("set OPENAI_REALTIME_MODEL and OPENAI_REALTIME_VOICE")
	}

	service, err := calls.NewWhatsAppCallService(config)

	if err != nil {
		log.Fatal(err)
	}

	session, err := calls.NewIncomingCallSession(os.Getenv("CALL_SENDER"), os.Getenv("CALL_ID"), "ru", time.Now().Add(30*time.Second))

	if err != nil {
		log.Fatal(err)
	}

	fsm := calls.NewCallStateMachine()
	fsm.Apply(session, calls.CallEventEnqueued)
	fsm.Apply(session, calls.CallEventDequeued)
	result, err := service.Execute(ctx, session, fsm.Apply)

	if err != nil {
		log.Fatal(err)
	}

	defer result.Close()
	log.Printf("state=%s reason=%s recording=%s", session.State, session.EndReason, result.RecordingPath())
}

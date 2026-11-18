// Outgoing voice conversation. This example makes a real call when run.
package main

import (
	"context"
	"github.com/green-api/whatsapp-chatbot-golang/calls"
	"log"
	"os"
	"os/exec"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	config := calls.ProviderConfig{APIURL: os.Getenv("GREEN_API_HOST"), IDInstance: os.Getenv("GREEN_API_ID"), APITokenInstance: os.Getenv("GREEN_API_TOKEN"), OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"), RealtimeModel: os.Getenv("OPENAI_REALTIME_MODEL"), RealtimeVoice: os.Getenv("OPENAI_REALTIME_VOICE")}

	if config.RealtimeModel == "" || config.RealtimeVoice == "" {
		log.Fatal("set OPENAI_REALTIME_MODEL and OPENAI_REALTIME_VOICE")
	}

	target := os.Getenv("CALL_TARGET")

	if target == "" {
		log.Fatal("set CALL_TARGET")
	}

	options := calls.DefaultCallOptions()

	if _, err := exec.LookPath(options.Recording.EncoderPath); err != nil {
		log.Printf("MP3 recording disabled: %s is unavailable; install FFmpeg or set options.Recording.EncoderPath", options.Recording.EncoderPath)
		options.Recording.Enabled = false
	}

	service, err := calls.NewWhatsAppCallService(config, options)

	if err != nil {
		log.Fatal(err)
	}

	session := calls.NewCallSession(target, target, "ru")
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

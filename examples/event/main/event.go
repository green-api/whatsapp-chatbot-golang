package main

import (
	"github.com/green-api/whatsapp-chatbot-golang"
	"log"
)

func main() {
	bot := whatsapp_chatbot_golang.NewBot("INSTANCE_ID", "TOKEN")

	bot.SetStartScene(StartScene{})

	bot.StartReceivingNotifications()
}

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing input messages")
	})

	bot.OutgoingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing outgoing messages")
	})

	bot.OutgoingMessageStatusHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing outgoing message statuses")
	})

	bot.IncomingBlockHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing chat blocking")
	})

	bot.IncomingCallHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing incoming calls")
	})

	bot.DeviceInfoHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing webhooks about the device status")
	})

	bot.StateInstanceChangedHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		log.Print("Logic for processing webhooks about changing the instance status")
	})
}

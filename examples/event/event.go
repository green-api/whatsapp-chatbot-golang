package event

import (
	"github.com/green-api/whatsapp_chatbot_golang"
)

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing input messages
	})

	bot.OutgoingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing outgoing messages
	})

	bot.OutgoingMessageStatusHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing outgoing message statuses
	})

	bot.IncomingBlockHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing chat blocking
	})

	bot.IncomingCallHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing incoming calls
	})

	bot.DeviceInfoHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing webhooks about the device status
	})

	bot.StateInstanceChangedHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		//Logic for processing webhooks about changing the instance status
	})
}

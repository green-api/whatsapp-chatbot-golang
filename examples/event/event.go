package event

import cb "whatsapp_chatbot_golang/chatbot"

type StartScene struct {
}

func (s StartScene) Start(bot *cb.Bot) {
	bot.IncomingMessageHandler(func(notification *cb.Notification) {
		//Logic for processing input messages
	})

	bot.OutgoingMessageHandler(func(notification *cb.Notification) {
		//Logic for processing outgoing messages
	})

	bot.OutgoingMessageStatusHandler(func(notification *cb.Notification) {
		//Logic for processing outgoing message statuses
	})

	bot.IncomingBlockHandler(func(notification *cb.Notification) {
		//Logic for processing chat blocking
	})

	bot.IncomingCallHandler(func(notification *cb.Notification) {
		//Logic for processing incoming calls
	})

	bot.DeviceInfoHandler(func(notification *cb.Notification) {
		//Logic for processing webhooks about the device status
	})

	bot.StateInstanceChangedHandler(func(notification *cb.Notification) {
		//Logic for processing webhooks about changing the instance status
	})
}

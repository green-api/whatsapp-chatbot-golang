package webhook

import (
	"github.com/green-api/whatsapp-chatbot-golang"
)

func Start() {
	bot := whatsapp_chatbot_golang.NewBot("INSTANCE_ID", "TOKEN")

	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"text": {"test"}}) {
			message.AnswerWithText("Well done! You have write \"test\".")
		} else {
			message.AnswerWithText("Write \"test\"!")
		}
	})

	bot.StartListeningForWebhooks(6000, "/", "https://your-domain-that-forwards-webhooks-to-bot.com")
}

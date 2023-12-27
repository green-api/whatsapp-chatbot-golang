package base

import (
	"whatsapp_chatbot_golang/chatbot"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
		if message.Filter(map[string][]string{"text": {"test"}}) {
			message.AnswerWithText("Well done! You have write \"test\".")
		} else {
			message.AnswerWithText("Write \"test\"!")
		}
	})

	bot.StartReceivingNotifications()
}

package filter

import (
	"github.com/green-api/whatsapp-chatbot-golang"
)

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"text": {"1"}}) {
			message.AnswerWithText("This message text equals \"1\"")
		}

		if message.Filter(map[string][]string{"text_regex": {"\\d+"}}) {
			message.AnswerWithText("This message has only digits!")
		}

		if message.Filter(map[string][]string{"text_regex": {"6"}}) {
			message.AnswerWithText("This message contains \"6\" in the text")
		}

		if message.Filter(map[string][]string{"text": {"hi"}, "messageType": {"textMessage", "extendedTextMessage"}}) {
			message.AnswerWithText("This message is a \"textMessage\" or \"extendedTextMessage\", and text equals \"hi\"")
		}
	})
}

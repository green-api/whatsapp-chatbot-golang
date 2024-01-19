package echo

import (
	"github.com/green-api/whatsapp-chatbot-golang"
)

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		text, _ := message.Text()
		message.AnswerWithText(text)
	})
}

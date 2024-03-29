package main

import (
	"github.com/green-api/whatsapp-chatbot-golang"
)

func main() {
	bot := whatsapp_chatbot_golang.NewBot("INSTANCE_ID", "TOKEN")

	bot.SetStartScene(StartScene{})

	bot.StartReceivingNotifications()
}

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		text, _ := message.Text()
		message.AnswerWithText(text)
	})
}

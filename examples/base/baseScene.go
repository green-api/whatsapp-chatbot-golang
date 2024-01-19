package base

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
		if message.Filter(map[string][]string{"text": {"test"}}) {
			message.AnswerWithText("Well done! You have write \"test\".")
			message.AnswerWithText("Now write \"second scene\"")
			message.ActivateNextScene(SecondScene{})
		} else {
			message.AnswerWithText("Write \"test\"!")
		}
	})
}

type SecondScene struct {
}

func (s SecondScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"text": {"second scene"}}) {
			message.AnswerWithText("Well done! You have write \"second scene\".")
			message.ActivateNextScene(StartScene{})
		} else {
			message.AnswerWithText("This is second scene write \"second scene\"!")
		}
	})
}

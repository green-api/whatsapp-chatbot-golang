package base

import (
	"whatsapp_chatbot_golang/chatbot"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.SetStartScene(StartScene{})

	bot.StartReceivingNotifications()
}

type StartScene struct {
}

func (s StartScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
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

func (s SecondScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
		if message.Filter(map[string][]string{"text": {"second scene"}}) {
			message.AnswerWithText("Well done! You have write \"second scene\".")
			message.ActivateNextScene(StartScene{})
		} else {
			message.AnswerWithText("This is second scene write \"second scene\"!")
		}
	})
}

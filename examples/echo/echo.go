package echo

import cb "whatsapp_chatbot_golang/chatbot"

type StartScene struct {
}

func (s StartScene) Start(bot *cb.Bot) {
	bot.IncomingMessageHandler(func(message *cb.Notification) {
		text, _ := message.Text()
		message.AnswerWithText(text)
	})
}

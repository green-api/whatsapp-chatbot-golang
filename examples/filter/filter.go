package filter

import cb "whatsapp_chatbot_golang/chatbot"

type StartScene struct {
}

func (s StartScene) Start(bot *cb.Bot) {
	bot.IncomingMessageHandler(func(message *cb.Notification) {
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

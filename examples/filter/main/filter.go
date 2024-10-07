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
		if message.Filter(map[string][]string{"text": {"1"}}) {
			// text = 1
			message.AnswerWithText("This message text equals \"1\"")
		} else if message.Filter(map[string][]string{"text_regex": {"^[0-9]+$"}}) {
			// text = [0 ... 9]
			message.AnswerWithText("This message has only digits!")
		} else if message.Filter(map[string][]string{"text": {"hi"}, "messageType": {"textMessage", "extendedTextMessage"}}) {
			// text = hi
			message.AnswerWithText("This message is a \"textMessage\" or \"extendedTextMessage\", and text equals \"hi\"")
		} else if message.Filter(map[string][]string{"text_regex": {"^[A-Za-z]+$"}}) {
			// text = A-Za-z
			message.AnswerWithText("This message contains only letters and no digits.")
		} else {
			// other cases
			typeMessage := message.Body["messageData"].(map[string]interface{})["typeMessage"].(string)
			if typeMessage == "textMessage" {
				text := message.Body["messageData"].(map[string]interface{})["textMessageData"].(map[string]interface{})["textMessage"].(string)
				message.AnswerWithText("You wrote: \"" + text + "\"")
			} else if typeMessage == "extendedTextMessage" {
				text := message.Body["messageData"].(map[string]interface{})["extendedTextMessageData"].(map[string]interface{})["text"].(string)
				message.AnswerWithText("You wrote: \"" + text + "\"")
			} else {
				message.AnswerWithText("You wrote a message with type: \"" + typeMessage + "\"")
			}

		}


	})
}

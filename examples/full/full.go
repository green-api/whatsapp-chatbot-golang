package full

import (
	"github.com/green-api/whatsapp_chatbot_golang"
)

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		if notification.Filter(map[string][]string{"text": {"/start"}}) {
			notification.AnswerWithText(`Привет! Этот бот использует различные методы API.
Пожалуйста выберите метод:
1. SendMessage()
2. SendFileByUrl()
3. SendPoll()
4. SendContact()
5. SendLocation()
Пришлите номер пункта одной цифрой.`)
			notification.ActivateNextScene(PickMethodScene{})
		} else {
			notification.AnswerWithText("Пожалуйста введите команду /start.")
		}
	})
}

type PickMethodScene struct {
}

func (s PickMethodScene) Start(bot *whatsapp_chatbot_golang.Bot) {

	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"text": {"1"}}) {
			message.AnswerWithText("Hello world!")
		}

		if message.Filter(map[string][]string{"text": {"2"}}) {
			message.AnswerWithText("Give me a link for a file, for example: https://th.bing.com/th/id/OIG.gq_uOPPdJc81e_v0XAei")
			message.ActivateNextScene(InputLinkScene{})
		}

		if message.Filter(map[string][]string{"text": {"3"}}) {
			message.AnswerWithPoll("Please choose a color:", false, []map[string]interface{}{
				{
					"optionName": "Red",
				},
				{
					"optionName": "Green",
				},
				{
					"optionName": "Blue",
				},
			})
		}

		if message.Filter(map[string][]string{"text": {"4"}}) {
			message.AnswerWithContact(map[string]interface{}{
				"phoneContact": 79001234568,
				"firstName":    "Артем",
				"middleName":   "Петрович",
				"lastName":     "Евпаторийский",
				"company":      "Велосипед",
			})
		}

		if message.Filter(map[string][]string{"text": {"5"}}) {
			message.AnswerWithLocation("House", "Cdad. de La Paz 2969, Buenos Aires", -34.5553558, -58.4642510)
		}

		if !message.Filter(map[string][]string{"text_regex": {"\\d+"}}) {
			message.AnswerWithText("Ответ должен содержать только цифры!")
		}
	})
}

type InputLinkScene struct {
}

func (s InputLinkScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"regex": {"^https://[^\\s]+$"}}) {
			text, _ := message.Text()
			message.AnswerWithUrlFile(text, "testFile", "This is your file!")
			message.ActivateNextScene(PickMethodScene{})
		} else {
			message.AnswerWithText("Ссылка не должна содержать пробелы и должна начинаться на https://")
		}
	})
}

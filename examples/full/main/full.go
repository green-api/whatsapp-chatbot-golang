package main

import (
	"net/http"

	greenapi "github.com/green-api/whatsapp-api-client-golang-v2"
	whatsapp_chatbot_golang "github.com/green-api/whatsapp-chatbot-golang"
)

func main() {
	bot := whatsapp_chatbot_golang.NewBot("INSTANCE_ID", "TOKEN")

	bot.SetStartScene(StartScene{})

	bot.StartReceivingNotifications()
}

type StartScene struct {
}

func (s StartScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(notification *whatsapp_chatbot_golang.Notification) {
		if notification.Filter(map[string][]string{"text": {"/start"}}) {
			notification.AnswerWithText("Привет! Этот бот использует различные методы API.\n" +
				"Пожалуйста выберите метод:\n" +
				"1. SendMessage()\n" +
				"2. SendFileByUrl()\n" +
				"3. SendPoll()\n" +
				"4. SendContact()\n" +
				"5. SendLocation()\n" +
				"6. SendInteractiveButtons()\n" +
				"7. SendInteractiveButtonsReply()\n" +
				"Пришлите номер пункта одной цифрой.")
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
		} else if message.Filter(map[string][]string{"text": {"2"}}) {
			message.AnswerWithText("Give me a link...")
			message.ActivateNextScene(InputLinkScene{})
		} else if message.Filter(map[string][]string{"text": {"3"}}) {
			message.AnswerWithPoll("Please choose a color:", false, []string{
				"Red",
				"Green",
				"Blue",
			})
		} else if message.Filter(map[string][]string{"text": {"4"}}) {
			message.AnswerWithContact(greenapi.Contact{
				PhoneContact: 79001234568,
				FirstName:    "Артем",
				MiddleName:   "Петрович",
				LastName:     "Евпаторийский",
				Company:      "Велосипед",
			})
		} else if message.Filter(map[string][]string{"text": {"5"}}) {
			message.AnswerWithLocation("House", "Cdad. de La Paz 2969, Buenos Aires", -34.5553558, -58.4642510)
		} else if message.Filter(map[string][]string{"text": {"6"}}) {
			message.AnswerWithButtons("Это сообщение с интерактивными кнопками.", []greenapi.InteractiveReplyButton{
				{ButtonId: "btn1", ButtonText: "Очень удобно!"},
				{ButtonId: "btn2", ButtonText: "А какие ещё кнопки есть?"},
			})
		} else if message.Filter(map[string][]string{"text": {"7"}}) || message.Filter(map[string][]string{"text": {"А какие ещё кнопки есть?"}}) {
			message.AnswerWithInteractiveButtons(
				"Вот кнопки с расширенным функционалом:",
				[]greenapi.InteractiveButton{
					{
						Type:       "url",
						ButtonId:   "btn_url",
						ButtonText: "Подробнее о кнопках",
						URL:        "https://green-api.com/docs/api/sending/SendInteractiveButtons/",
					},
					{
						Type:       "copy",
						ButtonId:   "btn_copy",
						ButtonText: "Копировать код",
						CopyCode:   "GREEN-API-2026",
					},
					{
						Type:        "call",
						ButtonId:    "btn_call",
						ButtonText:  "Номер технической поддержки",
						PhoneNumber: "79993331223",
					},
				},
				"Заголовок меню",
				"Нижний текст (footer)",
			)
		} else if message.Filter(map[string][]string{"text": {"Очень удобно!"}}) {
			message.AnswerWithText("Рад, что вам понравилось! \n Подробнее о методе: https://green-api.com/docs/api/sending/SendInteractiveButtonsReply/")
		} else {
			if !message.Filter(map[string][]string{"text_regex": {"\\d+"}}) {
				message.AnswerWithText("Пожалуйста, выберите пункт меню от 1 до 7.")
			}
		}
	})
}

type InputLinkScene struct {
}

func (s InputLinkScene) Start(bot *whatsapp_chatbot_golang.Bot) {
	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"regex": {"^https://[^\\s]+$"}}) {
			text, _ := message.Text()

			resp, err := http.Get(text)
			if err != nil {
				message.AnswerWithText("URL недоступен, пожалуйста, попробуйте другую ссылку.")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				message.AnswerWithUrlFile(text, "testFile", "This is your file!")
				message.ActivateNextScene(PickMethodScene{})
			} else {
				message.AnswerWithText("URL недоступен, пожалуйста, попробуйте другую ссылку.")
			}
		} else {
			message.AnswerWithText("Ссылка не должна содержать пробелы и должна начинаться на https://")
		}
	})
}

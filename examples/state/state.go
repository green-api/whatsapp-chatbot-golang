package state

import (
	"whatsapp_chatbot_golang/chatbot"
)

type StartScene struct {
}

func (s StartScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
		if notification.Filter(map[string][]string{"text": {"/start"}}) {
			notification.AnswerWithText("Привет! Этот бот - пример использования состояния.\nПожалуйста введите логин:")
			notification.ActivateNextScene(LoginScene{})
		} else {
			notification.AnswerWithText("Пожалуйста введите команду /start.")
		}
	})
}

type LoginScene struct {
}

func (s LoginScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
		login, err := notification.Text()
		if err != nil || len(login) > 12 || len(login) < 6 {
			notification.AnswerWithText("Выберите логин от 6 до 12 символов!")
		} else {
			notification.UpdateStateData(map[string]interface{}{"login": login})
			notification.ActivateNextScene(PasswordScene{})
			notification.AnswerWithText("Ваш логин " + notification.GetStateData()["login"].(string) + " - успешно сохранен.\nПридумайте пароль:")
		}
	})
}

type PasswordScene struct {
}

func (s PasswordScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
		password, err := notification.Text()
		if err != nil || len(password) > 16 || len(password) < 8 {
			notification.AnswerWithText("Выберите пароль от 8 до 16 символов!")
		} else {
			notification.UpdateStateData(map[string]interface{}{"password": password})
			notification.ActivateNextScene(StartScene{})
			notification.AnswerWithText("Успех! Ваш логин: " + notification.GetStateData()["login"].(string) + "\nВаш пароль: " + notification.GetStateData()["password"].(string))
		}
	})
}

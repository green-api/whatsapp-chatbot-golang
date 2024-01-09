# whatsapp-chatbot-golang

whatsapp-chatbot-golang - библиотека для интеграции с мессенджером WhatsApp через API
сервиса [green-api.com](https://green-api.com/). Чтобы воспользоваться библиотекой, нужно получить регистрационный токен
и ID аккаунта в [личном кабинете](https://console.green-api.com/). Есть бесплатный тариф аккаунта разработчика.

## API

Документация к REST API находится по [ссылке](https://green-api.com/docs/api/). Библиотека является обёрткой к REST API,
поэтому документация по ссылке выше применима и к самой библиотеке.

## Авторизация

Чтобы отправить сообщение или выполнить другие методы GREEN API, аккаунт WhatsApp в приложении телефона должен быть в
авторизованном состоянии. Для авторизации аккаунта перейдите в [личный кабинет](https://console.green-api.com/) и
сканируйте QR-код с использованием приложения WhatsApp.

## Установка

Не забудьте создать модуль:

```shell
go mod init example
```

Установка:

```shell
go get github.com/green-api/whatsapp-chatbot-golang
```

## Импорт

```
import (
	"github.com/green-api/whatsapp-chatbot-golang/"
)
```

## Примеры

### Как настроить инстанс

Создать инстанс можно в личном кабинете по [ссылке](https://console.green-api.com/). Нажмите создать и выберите тариф.
Чтобы начать получать входящие уведомления, нужно настроить инстанс. Открываем страницу личного кабинета
по [ссылке](https://console.green-api.com/instanceList). Выбираем инстанс из списка и кликаем на него. Нажимаем **Изменить**. В
категории **Уведомления** включаем все вебхуки которые необходимо получать.

### Как инициализировать объект

Для инициации бота нужно воспользоваться методом `NewBot` из библиотеки и указать номер инстанса и токен из личного кабинета.

```
bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")
```

Обратите внимание, что ключи можно получать из переменных среды:

```
IDInstance := os.Getenv("ID_INSTANCE")
APITokenInstance := os.Getenv("API_TOKEN_INSTANCE")
```

### Как начать получать сообщения и отвечать на них

Чтобы начать получать уведомления, необходимо вызвать у бота метод `bot.StartReceivingNotifications()`.
Но перед этим необходимо добавить обработчик, это можно сделать двумя способами. Вы можете сделать это сразу в функции `main` как в примере `base`.

Ссылка на пример: [base.go](../examples/base/base.go).

```go
package base

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
		if message.Filter(map[string][]string{"text": {"test"}}) {
			message.AnswerWithText("Well done! You have write \"test\".")
		} else {
			message.AnswerWithText("Write \"test\"!")
		}
	})

	bot.StartReceivingNotifications()
}
```

Если у вас сложные вложенные сценарии, лучше использовать сцены как в примере `baseScene`. 
Пользоваться сценами просто - достаточно вынести логику бота в отдельную структуру, которая реализовывает интерфейс `Scene`, и добавить ее в бот методом `bot.SetStartScene(StartScene{})`.
Стартовая сцена может вызвать следующую с помощью метода `message.ActivateNextScene(NextScene{})`, тогда следующий вебхук попадет уже в новую сцену, это позволит разделить бота на отдельные части и сделает код более читаемым и редактируемым:

```go
package base

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
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
```

Если вам нужно чтобы при создании нового состояния, оно уже имело некоторые дефолтные значения, необходимо изменить поле `InitData` у структуры `StateManager`.
В стандартной имплементации `MapStateManager` это делается так:

```go
package main

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
	"github.com/green-api/whatsapp_chatbot_golang/examples/full"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.StateManager = chatbot.NewMapStateManager(
		map[string]interface{}{
			"defaultField1": "defaultValue1",
			"defaultField2": "defaultValue2",
			"defaultField3": "defaultValue3",
		})

	bot.SetStartScene(full.StartScene{})

	bot.StartReceivingNotifications()
}
```

Обратите внимание, что во время выполнения запросов могут возникать ошибки, чтобы ваша программа не прерывалась из-за них,
вам необходимо обрабатывать ошибки. Все ошибки библиотеки отправляются в канал `ErrorChannel`, вы можете обработать их например таким способом:

```go
package main

import (
	"fmt"
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
	"github.com/green-api/whatsapp_chatbot_golang/examples/full"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.SetStartScene(full.StartScene{})

    //Все ошибки будут просто выводиться в консоль
	go func() {
		select {
		case err := <-bot.ErrorChannel:
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	bot.StartReceivingNotifications()
}
```

### Как получать другие уведомления и обрабатывать тело уведомления

Получать можно не только входящие сообщения, но и исходящие, а так же их статусы и любые другие типы веб хуков.
Для этого просто добавьте в сцену или в функцию main новый обработчик. В каждой сцене может быть несколько обработчиков.

Ссылка на пример: [event.go](../examples/event/event.go).

```go
package event

import cb "github.com/green-api/whatsapp_chatbot_golang/chatbot"

type StartScene struct {
}

func (s StartScene) Start(bot *cb.Bot) {
	bot.IncomingMessageHandler(func(notification *cb.Notification) {
		//Логика обработки входящих сообщений
	})

	bot.OutgoingMessageHandler(func(notification *cb.Notification) {
		//Логика обработки исходящих сообщений
	})

	bot.OutgoingMessageStatusHandler(func(notification *cb.Notification) {
		//Логика обработки статусов исходящих сообщений
	})

	bot.IncomingBlockHandler(func(notification *cb.Notification) {
		//Логика обработки блокировок чатов
	})

	bot.IncomingCallHandler(func(notification *cb.Notification) {
		//Логика обработки входящих звонков
	})

	bot.DeviceInfoHandler(func(notification *cb.Notification) {
		//Логика обработки вебхуков о статусе устройства
	})

	bot.StateInstanceChangedHandler(func(notification *cb.Notification) {
		//Логика обработки вебхуков о смене статуса инстанса
	})
}
```

### Как фильтровать входящие сообщения

Фильтрация по типу вебхука происходит автоматически на уровне создания обработчика, пример - [event.go](../examples/event/event.go).
Другие типы фильтров реализованы с помощью метода `Filter` который принимает в качестве параметра `map[string][]string{}`.
Ключом данной карты служит строка с именем параметра по которому будет происходить фильтрация, значение карты - срез с набором ожидаемых значений.
Если в фильтре по параметру несколько ожидаемых значений, то метод возвращает `true` если хотя бы одно ожидаемое значение совпадает с полем вебхука. 
Если метод фильтрует одновременно несколько параметров, то метод возвращает `true` только если все параметры прошли проверку.

| Имена параметров для фильтрации | Описание                                                                                                     |
|---------------------------------|--------------------------------------------------------------------------------------------------------------|
| `text`                          | Фильтр по тексту сообщения, если хоть один из ожидаемых значений совпадает, возвращает `true`                |
| `text_regex`                    | Фильтр по тексту сообщения, но по regex паттерну, если хотябы один паттерн в срезе подходит, возвращает true |
| `sender`                        | Возвращает `true`, если хотя бы одно ожидаемое значение равно идентификатору отправителя сообщения           |
| `chatId`                        | Возвращает `true`, если хотя бы одно ожидаемое значение равно идентификатору чата сообщения                  |
| `messageType`                   | Возвращает `true`, хотя бы одно ожидаемое значение равно значению поля `messageType` в вебхуке               |

Ссылка на пример: [filter.go](../examples/filter/filter.go).

```go
package filter

import cb "github.com/green-api/whatsapp_chatbot_golang/chatbot"

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

```

### Как управлять состоянием пользователя

По умолчанию в данной библиотеке состояние хранится в карте типа `map[string]interface{}{}`.
В качестве ключа может быть любая строка в качестве значения, любой объект.
Идентификатором состояния является идентификатор чата, то есть у каждого чата будет отдельное состояние.
Чтобы управлять состоянием, нужно использовать методы структуры `Notification`:

| Метод менеджера        | Описание                                     |
|------------------------|----------------------------------------------|
| `ActivateNextScene()`  | Активирует выбранную сцену.                  |
| `GetCurrentScene()`    | Возвращает текущую сцену.                    |
| `GetStateData()`       | Возвращает данные состояния выбранного чата. |
| `SetStateData()`       | Заменяет данные состояния выбранного чата.   |
| `UpdateStateData()`    | Обновляет данные состояния выбранного чата.  |

> Вебхуки типа incomingBlock, deviceInfo, stateInstanceChanged не привязаны к чату, поэтому не имеют собственного состояния.
> Если вы хотите взаимодействовать с состояниями других чатов, отличных от чата обрабатываемого вебхука, вы можете использовать методы структуры `StateManager` напрямую. Методы `StateManager` делают тоже что и методы структуры `Notification`, но они ожидают дополнительный параметр `stateId`.

В качестве примера был создан простой бот для имитации регистрации пользователя.

Ссылка на пример: [state.go](../examples/state/state.go).

```go
package state

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
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
```

Если вам нужно чтобы при создании нового состояния, оно уже имело некоторые дефолтные значения, необходимо изменить поле `InitData` у структуры `StateManager`. 
В стандартной имплементации `MapStateManager` это делается так:

```go
package main

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
	"github.com/green-api/whatsapp_chatbot_golang/examples/full"
)

func main() {
	bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")

	bot.StateManager = chatbot.NewMapStateManager(
		map[string]interface{}{
			"defaultField1": "defaultValue1",
			"defaultField2": "defaultValue2",
			"defaultField3": "defaultValue3",
		})

	bot.SetStartScene(full.StartScene{})

	bot.StartReceivingNotifications()
}
```

### Пример бота

В качестве примера был создан бот демонстрирующий отправку методов структуры `Notification`.

Запуск бота происходит командой - /start
После запуска необходимо выбрать метод из меню, и бот выполнит его.

Ссылка на пример: [full,go](../examples/full/full.go).

Стартовая сцена ждет команду `/start`, после чего отправляет меню и активирует следующую сцену `PickMethodScene`.
`PickMethodScene` ждет ответа пользователя и выполняет выбранный метод.
Если пользователь выбрал `SendFileByUrl()`, то бот запустит сцену `InputLinkScene`, в которой попросит ссылку на файл и отправит файл, если ссылка валидна.

```go
package full

import (
	"github.com/green-api/whatsapp_chatbot_golang/chatbot"
)

type StartScene struct {
}

func (s StartScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
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

func (s PickMethodScene) Start(bot *chatbot.Bot) {

	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
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

func (s InputLinkScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(message *chatbot.Notification) {
		if message.Filter(map[string][]string{"regex": {"^https://[^\\s]+$"}}) {
			text, _ := message.Text()
			message.AnswerWithUrlFile(text, "testFile", "This is your file!")
			message.ActivateNextScene(PickMethodScene{})
		} else {
			message.AnswerWithText("Ссылка не должна содержать пробелы и должна начинаться на https://")
		}
	})
}
```

## Документация по методам сервиса

[Документация по методам сервиса](https://green-api.com/docs/api/)

## Лицензия

Лицензировано на условиях [
Creative Commons Attribution-NoDerivatives 4.0 International (CC BY-ND 4.0)
](https://creativecommons.org/licenses/by-nd/4.0/).
[LICENSE](../LICENSE).
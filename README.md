# whatsapp-chatbot-golang

[Документация на русском языке.](docs/README_RU.md)

whatsapp-chatbot-golang - library for integration with WhatsApp messenger via API
service [greenapi.com](https://greenapi.com/). To use the library, you need to obtain a registration token
and account ID in [personal account](https://console.greenapi.com/). There is a free developer account plan.

## API

Documentation for the REST API can be found at [link](https://greenapi.com/docs/api/). The library is a wrapper for the
REST API,
therefore the documentation in the link above also applies to the library itself.

## Authorization

To send a message or perform other GREEN API methods, the WhatsApp account in the phone app must be in
authorized state. To authorize your account, go to [personal account](https://console.greenapi.com/) and
scan the QR code using the WhatsApp application.

## Installation

Don't forget to create a module:

```shell
go mod init example
```

Installation:

```shell
go get github.com/green-api/whatsapp-chatbot-golang
```

## Import

```
import (
"github.com/green-api/whatsapp-chatbot-golang/"
)
```

## Setup

Before launching the bot you should enable incoming notifications in instance settings by using <a href="https://green-api.com/en/docs/api/account/SetSettings/">SetSettings method</a>.

```json
"incomingWebhook": "yes",
"outgoingMessageWebhook": "yes",
"outgoingAPIMessageWebhook": "yes",
```

## Examples

### How to set up an instance

You can create an instance in your personal account using [link](https://console.greenapi.com/). Click create and
select a tariff.
To start receiving incoming notifications, you need to configure your instance. Open your personal account page
via [link](https://console.greenapi.com/instanceList). Select an instance from the list and click on it. Click **Change**. In
**Notifications** category includes all webhooks that need to be received.

### How to initialize an object

To initiate a bot, you need to use the `NewBot` method from the library and specify the instance number and token from
your personal account.

```
bot := chatbot.NewBot("INSTANCE_ID", "TOKEN")
```

Note that keys can be obtained from environment variables:

```
IDInstance := os.Getenv("ID_INSTANCE")
APITokenInstance := os.Getenv("API_TOKEN_INSTANCE")
```

### How to start receiving and responding to messages

To start receiving notifications, you need to call the bot.StartReceivingNotifications() method on the bot.
But before that you need to add a handler; this can be done in two ways. You can do this directly in the `main` function
as in the `base` example:

Link to example: [base.go](examples/base/base.go).

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

Or you can to call the bot.StartListeningForWebhooks(port int, endpoint, webhookUrl string) method on the bot (You still need to add a handler beforehand).
The bot.StartListeningForWebhooks requires 3 arguments to run:
1. port (int): The port number on which the webhook listener will be started.
2. endpoint (string): The specific endpoint or URL pattern where the webhook listener will receive incoming requests.
3. webhookUrl (string): The URL to which webhook notifications will be sent. This URL should forward incoming requests to localhost:port/endpoint. If non-empty string is provided webhookUrl of the instance will be set to webhookUrl from the arguments, otherwise no changes to instance settings will be applied.
See example:

Link to example: [webhook.go](examples/webhook/webhook.go).

```go
package webhook

import (
	"github.com/green-api/whatsapp-chatbot-golang"
)

func Start() {
	bot := whatsapp_chatbot_golang.NewBot("INSTANCE_ID", "TOKEN")

	bot.IncomingMessageHandler(func(message *whatsapp_chatbot_golang.Notification) {
		if message.Filter(map[string][]string{"text": {"test"}}) {
			message.AnswerWithText("Well done! You have write \"test\".")
		} else {
			message.AnswerWithText("Write \"test\"!")
		}
	})

	bot.StartListeningForWebhooks(6000, "/", "https://your-domain-that-forwards-webhooks-to-bot.com")
}
```

Or if you have complex nested scripts, it is better to use Scenes as in the `baseScene` example.
Using Scenes is easy - put the bot logic into a separate structure that implements the `Scene` interface, and add it to
the bot using the `bot.SetStartScene(StartScene{})` method.
The starting scene can call the next one using the `message.ActivateNextScene(NextScene{})` method, then the next webhook will go into the new scene, this will allow you to divide the bot into separate parts and make the code more readable and editable:

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

If you need that when creating a new state, it already has some default values, you need to change the `InitData` field of the `StateManager` structure.
In the standard implementation of `MapStateManager` this is done like this:

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

Please note that errors may occur while executing queries so that your program does not break due to them, you need to handle errors. All library errors are sent to the `ErrorChannel` channel, you can handle them for example in this way:

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

	//All errors will simply be output to the console
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

### How to receive other notifications and handle the notification body

You can receive not only incoming messages, but also outgoing ones, as well as their statuses and any other types of web
hooks.
To do this, simply add a new handler to the scene or main function. Each scene can have multiple handlers.

Link to example: [event.go](examples/event/event.go).

```go
package event

import cb "github.com/green-api/whatsapp_chatbot_golang/chatbot"

type StartScene struct {
}

func (s StartScene) Start(bot *cb.Bot) {
	bot.IncomingMessageHandler(func(notification *cb.Notification) {
		//Logic for processing input messages
	})

	bot.OutgoingMessageHandler(func(notification *cb.Notification) {
		//Logic for processing outgoing messages
	})

	bot.OutgoingMessageStatusHandler(func(notification *cb.Notification) {
		//Logic for processing outgoing message statuses
	})

	bot.IncomingBlockHandler(func(notification *cb.Notification) {
		//Logic for processing chat blocking
	})

	bot.IncomingCallHandler(func(notification *cb.Notification) {
		//Logic for processing incoming calls
	})

	bot.DeviceInfoHandler(func(notification *cb.Notification) {
		//Logic for processing webhooks about the device status
	})

	bot.StateInstanceChangedHandler(func(notification *cb.Notification) {
		//Logic for processing webhooks about changing the instance status
	})
}
```

### Receive webhooks via HTTP API

You can get incoming webhooks (messages, statuses) via HTTP API requests in the similar way as the rest of the Green API methods are implemented. Herewith, the chronological order of the webhooks following is guaranteed in the sequence in which they were received FIFO. All incoming webhooks are stored in the queue and are expected to be received within 24 hours.

To get incoming webhooks, you have to sequentially call two methods <a href="https://green-api.com/en/docs/api/receiving/technology-http-api/ReceiveNotification/">ReceiveNotification</a> and <a href="https://green-api.com/en/docs/api/receiving/technology-http-api/DeleteNotofication/">DeleteNotification</a>. ReceiveNotification method receives an incoming webhook. DeleteNotification method confirms successful webhook receipt and processing. To learn more about the methods, refer to respective ReceiveNotification and DeleteNotification sections.

### How to filter incoming messages

Filtering by webhook type occurs automatically at the handler creation level, for
example - [event.go](examples/event/event.go).
Other types of filters are implemented using the `Filter` method, which takes `map[string][]string{}` as a parameter.
The key of this map is a string with the name of the parameter by which filtering will occur, the value of the map is a
slice with a set of expected values.
If there are several expected values in the filter for a parameter, then the method returns `true` if at least one
expected value matches the webhook field.
If a method filters several parameters at the same time, the method returns `true` only if all parameters pass the test.

| Names of parameters for filtering | Description                                                                                                   |
|-----------------------------------|---------------------------------------------------------------------------------------------------------------|
| `text`                            | Filter by message text, if at least one of the expected values matches, returns `true`                        |
| `text_regex`                      | Filter by message text, but by regex pattern, if at least one pattern in the slice matches, returns true      |
| `sender`                          | Returns `true` if at least one expected value equals the message sender ID                                    |
| `chatId`                          | Returns `true` if at least one expected value equals the message's chat ID                                    |
| `messageType`                     | Returns `true` if at least one expected value is equal to the value of the `messageType` field in the webhook |

Link to example: [filter.go](examples/filter/filter.go).

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

### How to manage user state

By default, in this library the state is stored in a map of type `map[string]interface{}{}`.
The key can be any string, any object can be the value.
The state ID is the chat ID, meaning each chat will have a separate state.
To manage the state, you need to use the methods of the `Notification` structure:

| Manager method          | Description                                   |
|-------------------------|-----------------------------------------------|
| `ActivateNextScene()`   | Activates the selected scene.                 |
| `GetCurrentScene()`     | Returns the current scene.                    |
| `GetStateData()`        | Returns the status data of the selected chat. |
| `SetStateData()`        | Replaces the state data of the selected chat. |
| `UpdateStateData()`     | Updates the status data of the selected chat. |

> Webhooks like incomingBlock, deviceInfo, stateInstanceChanged are not tied to the chat, so they do not have their own
> state.
> If you want to interact with the states of other chats other than the chat of the webhook being processed, you can use
> the methods of the `StateManager` structure directly. The `StateManager` methods do the same as the `Notification`
> structure methods, but they expect an additional `stateId` parameter.

As an example, a simple bot was created to simulate user registration.

Link to example: [state.go](examples/state/state.go).

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
			notification.AnswerWithText("Hi! This bot is an example of using state.\nPlease enter your login:")
			notification.ActivateNextScene(LoginScene{})
		} else {
			notification.AnswerWithText("Please enter the /start command.")
		}
	})
}

type LoginScene struct {
}

func (s LoginScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
		login, err := notification.Text()
		if err != nil || len(login) > 12 || len(login) < 6 {
			notification.AnswerWithText("Select a login from 6 to 12 characters!")
		} else {
			notification.UpdateStateData(map[string]interface{}{"login": login})
			notification.ActivateNextScene(PasswordScene{})
			notification.AnswerWithText("Your login " + notification.GetStateData()["login"].(string) + " - successfully saved.\nCreate a password:")
		}
	})
}

type PasswordScene struct {
}

func (s PasswordScene) Start(bot *chatbot.Bot) {
	bot.IncomingMessageHandler(func(notification *chatbot.Notification) {
		password, err := notification.Text()
		if err != nil || len(password) > 16 || len(password) < 8 {
			notification.AnswerWithText("Choose a password between 8 and 16 characters!")
		} else {
			notification.UpdateStateData(map[string]interface{}{"password": password})
			notification.ActivateNextScene(StartScene{})
			notification.AnswerWithText("Success! Your login: " + notification.GetStateData()["login"].(string) + "\nYour password: " + notification.GetStateData()["password"].(string))
		}
	})
}
```

If you need that when creating a new state, it already has some default values, you need to change the `InitData` field
of the `StateManager` structure.
In the standard implementation of `MapStateManager` this is done like this:

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

### Bot example

As an example, a bot was created that demonstrates sending methods of the `Notification` structure.

The bot is started with the command - /start
After launching, you need to select a method from the menu, and the bot will execute it.

Link to example: [full,go](examples/full/full.go).

The start scene waits for the `/start` command, after which it sends the menu and activates the next `PickMethodScene`.
`PickMethodScene` waits for the user's response and executes the selected method.
If the user selected `SendFileByUrl()`, then the bot will launch the `InputLinkScene` scene, in which it will ask for a
link to the file and send the file if the link is valid.

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
			notification.AnswerWithText(`Hi! This bot uses various API methods.
Please select a method:
1. SendMessage()
2. SendFileByUrl()
3. SendPoll()
4. SendContact()
5. SendLocation()
Send the item number in one digit.`)
			notification.ActivateNextScene(PickMethodScene{})
		} else {
			notification.AnswerWithText("Please enter the /start command.")
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
				"firstName":    "Artem",
				"middleName":   "Petrovich",
				"lastName":     "Evpatoria",
				"company":      "Bicycle",
			})
		}

		if message.Filter(map[string][]string{"text": {"5"}}) {
			message.AnswerWithLocation("House", "Cdad. de La Paz 2969, Buenos Aires", -34.5553558, -58.4642510)
		}

		if !message.Filter(map[string][]string{"text_regex": {"\\d+"}}) {
			message.AnswerWithText("Answer must contain only numbers!")
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
			message.AnswerWithText("The link must not contain spaces and must begin with https://")
		}
	})
}

```

## Documentation on service methods

[Documentation on service methods](https://greenapi.com/docs/api/)

## License

Licensed under [
Creative Commons Attribution-NoDerivatives 4.0 International (CC BY-ND 4.0)
](https://creativecommons.org/licenses/by-nd/4.0/).
[LICENSE](LICENSE).

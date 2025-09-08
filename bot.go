package whatsapp_chatbot_golang

import (
	"encoding/json"
	"fmt"
	greenapi "github.com/green-api/whatsapp-api-client-golang-v2"
	"log"
	"strings"
	"time"
)

type Bot struct {
	greenapi.GreenAPI
	CleanNotificationQueue bool
	StateManager
	Publisher
	isStart      bool
	ErrorChannel chan error
}

func NewBot(IDInstance string, APITokenInstance string) *Bot {
	bot := &Bot{
		GreenAPI: greenapi.GreenAPI{
			APIURL:           "https://api.green-api.com",
			MediaURL:         "https://media.green-api.com",
			IDInstance:       IDInstance,
			APITokenInstance: APITokenInstance,
		},
		CleanNotificationQueue: true,
		StateManager:           NewMapStateManager(map[string]interface{}{}),
		Publisher:              Publisher{},
		ErrorChannel:           make(chan error, 1),
	}

	go func() {
		for err := range bot.ErrorChannel {
			resultStr := err.Error()
			ind := strings.Index(resultStr, ". Body")
			if ind != -1 {
				resultStr = resultStr[:ind]
			}
			if strings.Contains(resultStr, "403") {
				resultStr += " Forbidden (probably instance data is wrong or not specified)"
			} else if strings.Contains(resultStr, "500") {
				resultStr = ""
			}
			if resultStr != "" {
				log.Printf("Error: %v\n", resultStr)
			}
		}
	}()

	return bot
}

func (b *Bot) StartReceivingNotifications() {
	if b.CleanNotificationQueue {
		b.DeleteAllNotifications()
	}

	b.isStart = true
	log.Print("Bot Start receive webhooks")

	receiver := b.Receiving()
	for b.isStart == true {
		response, err := receiver.ReceiveNotification()
		if err != nil {
			b.ErrorChannel <- err
			time.Sleep(5000)
			continue
		}
		if response == nil || len(response.Body) == 0 || string(response.Body) == "null" {
			continue
		} else {
			var responseMapTopLevel map[string]interface{}
			errUnmarshal := json.Unmarshal(response.Body, &responseMapTopLevel)
			if errUnmarshal != nil {
				log.Printf("Error unmarshaling webhook top level: %v. Raw: %s", errUnmarshal, string(response.Body))
				b.ErrorChannel <- fmt.Errorf("failed to unmarshal webhook top level: %w", errUnmarshal)
				continue
			}

			actualBodyMap := responseMapTopLevel["body"].(map[string]interface{})
			receiptIdRaw := responseMapTopLevel["receiptId"]
			log.Printf("Webhook received - %+v", responseMapTopLevel)
			notification := NewNotification(actualBodyMap, b.StateManager, b.GreenAPI, &b.ErrorChannel)

			b.startCurrentScene(notification)

			receiptId := int(receiptIdRaw.(float64))

			_, err = receiver.DeleteNotification(receiptId)
			if err != nil {
				b.ErrorChannel <- err
				continue
			}
		}
	}
}

func (b *Bot) StopReceivingNotifications() {
	if b.isStart {
		b.isStart = false
		log.Print("Bot stopped")
	} else {
		log.Print("Bot already stopped!")
	}
}

func (b *Bot) DeleteAllNotifications() {
	log.Print("deleting notifications Start...")
	receiver := b.Receiving()
	for {
		response, errRecv := receiver.ReceiveNotification(greenapi.OptionalReceiveTimeout(2))
		if errRecv != nil {
			log.Printf("Error receiving during delete all: %v", errRecv)
			time.Sleep(5000)
			continue
		}

		if response == nil || len(response.Body) == 0 || string(response.Body) == "null" {
			log.Print("deleting notifications finished!")
			break
		} else {
			var responseMapTopLevel map[string]interface{}
			err := json.Unmarshal(response.Body, &responseMapTopLevel)
			if err != nil {
				log.Println(string(response.Body))
				continue
			}
			
			receiptIdRaw := responseMapTopLevel["receiptId"]
			receiptId := int(receiptIdRaw.(float64))

			_, errDel := receiver.DeleteNotification(receiptId)
			if errDel != nil {
				b.ErrorChannel <- errDel
			}
		}
	}
}

func (b *Bot) startCurrentScene(n *Notification) {
	currentState := n.StateManager.Get(n.StateId)
	if currentState == nil {
		currentState = n.StateManager.Create(n.StateId)
	}
	if n.GetCurrentScene() != nil {
		b.Publisher.clearAll()
		n.GetCurrentScene().Start(b)
	}

	b.Publisher.publish(n)
}

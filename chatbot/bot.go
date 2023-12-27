package chatbot

import (
	"github.com/green-api/whatsapp-api-client-golang/pkg/api"
	"log"
	"time"
)

type Bot struct {
	api.GreenAPI
	CleanNotificationQueue bool
	StateManager
	Publisher
	isStart      bool
	ErrorChannel chan error
}

func NewBot(IDInstance string, APITokenInstance string) *Bot {
	return &Bot{
		GreenAPI: api.GreenAPI{
			IDInstance:       IDInstance,
			APITokenInstance: APITokenInstance,
		},
		CleanNotificationQueue: true,
		StateManager:           NewMapStateManager(map[string]interface{}{}),
		Publisher:              Publisher{},
		ErrorChannel:           make(chan error, 1),
	}
}

func (b *Bot) StartReceivingNotifications() {
	if b.CleanNotificationQueue {
		b.DeleteAllNotifications()
	}

	b.isStart = true
	log.Print("Bot Start receive webhooks")

	for b.isStart == true {
		response, err := b.Methods().Receiving().ReceiveNotification()
		if err != nil {
			b.ErrorChannel <- err
			time.Sleep(5000)
			continue
		}

		if response["body"] == nil {
			log.Print("webhook queue is empty")
			continue

		} else {
			responseBody := response["body"].(map[string]interface{})
			notification := NewNotification(responseBody, b.StateManager, &b.GreenAPI, &b.ErrorChannel)

			b.startCurrentScene(notification)

			_, err := b.Methods().Receiving().DeleteNotification(int(response["receiptId"].(float64)))
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
	for {
		response, _ := b.Methods().Receiving().ReceiveNotification()

		if response["body"] == nil {
			log.Print("deleting notifications finished!")
			break

		} else {
			_, err := b.Methods().Receiving().DeleteNotification(int(response["receiptId"].(float64)))
			if err != nil {
				b.ErrorChannel <- err
			}
		}
	}
}

func (b *Bot) startCurrentScene(n *Notification) {
	currentState := n.StateManager.Get(n.StateId)
	if currentState == nil {
		currentState = n.StateManager.Create(n.StateId)
	}
	if currentState.getScene() != nil {
		b.Publisher.clearAll()
		n.GetCurrentScene().Start(b)
	}

	b.Publisher.publish(n)
}

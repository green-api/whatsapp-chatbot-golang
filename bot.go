package whatsapp_chatbot_golang

import (
	"fmt"
	"log"
	"time"

	"github.com/green-api/whatsapp-api-client-golang/pkg/api"
	webhook "github.com/green-api/whatsapp-api-webhook-server-golang/pkg"
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

func (b *Bot) StartListeningForWebhooks(port int, endpoint, webhookToken, webhookUrl string) {
	if b.CleanNotificationQueue {
		b.DeleteAllNotifications()
	}

	b.isStart = true
	log.Println("Bot Start receive webhooks")

	if webhookUrl != "" {
		log.Println("Setting webhook URL, it may tale up to 5 minutes. Please, be patient")
		b.Methods().Account().SetSettings(map[string]interface{}{
			"webhookUrl": webhookUrl,
		})
	}

	webhookListener := webhook.Webhook{
		Address:      fmt.Sprintf(":%d", port),
		Pattern:      endpoint,
		WebhookToken: webhookToken,
	}

	err := webhookListener.StartServer(b.handleWebhook)
	if err != nil {
		log.Println("Failed to handle webhook: " + err.Error())
	}
}

func (b *Bot) handleWebhook(body map[string]interface{}) {
	notification := NewNotification(body, b.StateManager, &b.GreenAPI, &b.ErrorChannel)
	b.startCurrentScene(notification)
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
	if n.GetCurrentScene() != nil {
		b.Publisher.clearAll()
		n.GetCurrentScene().Start(b)
	}

	b.Publisher.publish(n)
}

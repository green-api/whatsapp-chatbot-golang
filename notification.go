package whatsapp_chatbot_golang

import (
	"errors"
	"github.com/green-api/whatsapp-api-client-golang/pkg/api"
	"github.com/green-api/whatsapp_chatbot_golang/scene"
	"github.com/green-api/whatsapp_chatbot_golang/state"
)

type Notification struct {
	Body map[string]interface{}
	state.StateManager
	*api.GreenAPI
	StateId      string
	ErrorChannel *chan error
}

func NewNotification(body map[string]interface{}, stateManager state.StateManager, greenAPI *api.GreenAPI, errorChannel *chan error) *Notification {
	notification := Notification{Body: body, StateManager: stateManager, GreenAPI: greenAPI, StateId: "", ErrorChannel: errorChannel}
	notification.createStateId()
	return &notification
}

func (n *Notification) Text() (string, error) {
	if n.isIncomingMessage() || n.isOutgoingMessage() {
		typeMessage := n.Body["messageData"].(map[string]interface{})["typeMessage"].(string)

		if typeMessage == "textMessage" {
			return n.Body["messageData"].(map[string]interface{})["textMessageData"].(map[string]interface{})["textMessage"].(string), nil
		} else if typeMessage == "extendedTextMessage" {
			return n.Body["messageData"].(map[string]interface{})["extendedTextMessageData"].(map[string]interface{})["text"].(string), nil
		}
	}
	return "", errors.New("text not exist, typeMessage isn't textMessage or extendedTextMessage")
}

func (n *Notification) Sender() (string, error) {
	if n.isIncomingMessage() || n.isOutgoingMessage() {
		return n.Body["senderData"].(map[string]interface{})["chatId"].(string), nil
	}

	return "", errors.New("sender not found, it isn't message webhook")
}

func (n *Notification) ChatId() (string, error) {
	if n.isIncomingMessage() || n.isOutgoingMessage() {
		return n.Body["senderData"].(map[string]interface{})["sender"].(string), nil
	}

	return "", errors.New("chatId not found, it isn't message webhook")
}

func (n *Notification) MessageType() (string, error) {
	return n.Body["messageData"].(map[string]interface{})["typeMessage"].(string), nil
}

func (n *Notification) ActivateNextScene(scene scene.Scene) {
	n.StateManager.ActivateNextScene(n.StateId, scene)
}

func (n *Notification) GetCurrentScene() scene.Scene {
	return n.StateManager.GetCurrentScene(n.StateId)
}

func (n *Notification) GetStateData() map[string]interface{} {
	return n.StateManager.GetStateData(n.StateId)
}

func (n *Notification) SetStateData(newStateData map[string]interface{}) {
	n.StateManager.SetStateData(n.StateId, newStateData)
}

func (n *Notification) UpdateStateData(newStateData map[string]interface{}) {
	n.StateManager.UpdateStateData(n.StateId, newStateData)
}

func (n *Notification) createStateId() {
	if n.isIncomingMessage() {
		n.StateId = n.Body["senderData"].(map[string]interface{})["chatId"].(string)

	} else if n.isOutgoingMessage() {
		n.StateId = n.Body["senderData"].(map[string]interface{})["chatId"].(string)

	} else if n.isOutgoingMessageStatus() {
		n.StateId = n.Body["chatId"].(string)

	} else if n.isIncomingCall() {
		n.StateId = n.Body["from"].(string)
	}
}

package whatsapp_chatbot_golang

import (
	"errors"
	"fmt"
	"log"

	greenapi "github.com/green-api/whatsapp-api-client-golang-v2"
)

type Notification struct {
	Body map[string]interface{}
	StateManager
	greenapi.GreenAPI
	StateId      string
	ErrorChannel *chan error
}

func NewNotification(body map[string]interface{}, stateManager StateManager, greenAPI greenapi.GreenAPI, errorChannel *chan error) *Notification {
	notification := Notification{Body: body, StateManager: stateManager, GreenAPI: greenAPI, StateId: "", ErrorChannel: errorChannel}
	notification.createStateId()
	return &notification
}

func (n *Notification) Text() (string, error) {
	if !n.isIncomingMessage() && !n.isOutgoingMessage() {
		return "", errors.New("not a message webhook")
	}

	msgData, ok := n.Body["messageData"].(map[string]interface{})
	if !ok {
		return "", errors.New("messageData not found")
	}

	typeInterface, ok := msgData["typeMessage"]
	if !ok {
		return "", errors.New("typeMessage not found")
	}
	typeMessage := typeInterface.(string)

	switch typeMessage {
	case "textMessage":
		if data, ok := msgData["textMessageData"].(map[string]interface{}); ok {
			return data["textMessage"].(string), nil
		}

	case "extendedTextMessage":
		if data, ok := msgData["extendedTextMessageData"].(map[string]interface{}); ok {
			if val, ok := data["text"].(string); ok && val != "" {
				return val, nil
			}
			if desc, ok := data["description"].(string); ok {
				return desc, nil
			}
		}

	case "buttonsResponseMessage":
		if data, ok := msgData["buttonsResponseMessage"].(map[string]interface{}); ok {
			return data["selectedButtonText"].(string), nil
		}

	case "interactiveButtonsResponse":
		if data, ok := msgData["interactiveButtonsResponse"].(map[string]interface{}); ok {
			return data["selectedDisplayText"].(string), nil
		}

	case "templateButtonReplyMessage":
		if data, ok := msgData["templateButtonReplyMessage"].(map[string]interface{}); ok {
			return data["selectedDisplayText"].(string), nil
		}

	case "listResponseMessage":
		if data, ok := msgData["listResponseMessage"].(map[string]interface{}); ok {
			return data["title"].(string), nil
		}
	}

	log.Printf("Unknown or empty message type: %s. Body: %+v", typeMessage, msgData)
	return "", fmt.Errorf("text does not exist for typeMessage: %s", typeMessage)
}

func (n *Notification) Sender() (string, error) {
	if n.isIncomingMessage() || n.isOutgoingMessage() {
		return n.Body["senderData"].(map[string]interface{})["sender"].(string), nil
	}

	return "", errors.New("sender not found, it isn't message webhook")
}

func (n *Notification) ChatId() (string, error) {
	if n.isIncomingMessage() || n.isOutgoingMessage() {
		return n.Body["senderData"].(map[string]interface{})["chatId"].(string), nil
	}

	return "", errors.New("chatId not found, it isn't message webhook")
}

func (n *Notification) MessageType() (string, error) {
	return n.Body["messageData"].(map[string]interface{})["typeMessage"].(string), nil
}

func (n *Notification) ActivateNextScene(scene Scene) {
	n.StateManager.ActivateNextScene(n.StateId, scene)
}

func (n *Notification) GetCurrentScene() Scene {
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

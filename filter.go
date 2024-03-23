package whatsapp_chatbot_golang

import "regexp"

func (n *Notification) Filter(expectedParam map[string][]string) bool {
	if len(expectedParam) == 0 {
		return true
	}

	text, _ := n.Text()
	sender, _ := n.Sender()
	chatId, _ := n.ChatId()
	messageType, _ := n.MessageType()

	for key, values := range expectedParam {
		switch key {
		case "text":
			if !contains(values, text) {
				return false
			}
		case "text_regex":
			for _, pattern := range values {
				matched, _ := regexp.MatchString(pattern, text)
				if !matched {
					return false
				}
			}
		case "sender":
			if !contains(values, sender) {
				return false
			}
		case "chatId":
			if !contains(values, chatId) {
				return false
			}
		case "messageType":
			if !contains(values, messageType) {
				return false
			}
		}
	}
	return true
}

func contains(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

func (n *Notification) isIncomingMessage() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "incomingMessageReceived"
}

func (n *Notification) isPollUpdateMessage() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "pollUpdateMessage"
}

func (n *Notification) isOutgoingMessage() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "outgoingMessageReceived" || typeWebhook == "outgoingAPIMessageReceived"
}

func (n *Notification) isOutgoingMessageStatus() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "outgoingMessageStatus"
}

func (n *Notification) isStateInstanceChanged() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "stateInstanceChanged"
}

func (n *Notification) isIncomingCall() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "incomingCall"
}

func (n *Notification) isIncomingBlock() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "incomingBlock"
}

func (n *Notification) isDeviceInfo() bool {
	typeWebhook := n.Body["typeWebhook"].(string)
	return typeWebhook == "deviceInfo"
}

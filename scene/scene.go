package scene

import "github.com/green-api/whatsapp_chatbot_golang"

type Scene interface {
	Start(*whatsapp_chatbot_golang.Bot)
}

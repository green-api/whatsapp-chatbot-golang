package whatsapp_chatbot_golang

func (n *Notification) AnswerWithText(text string) map[string]interface{} {
	chatId := tryParseChatId(n)

	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendMessage(map[string]interface{}{
		"chatId":          chatId,
		"message":         text,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) AnswerWithUploadFile(filePath string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)

	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendFileByUpload(filePath, map[string]interface{}{
		"chatId":          chatId,
		"caption":         caption,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) AnswerWithUrlFile(urlFile string, filename string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendFileByUrl(map[string]interface{}{
		"chatId":          chatId,
		"urlFile":         urlFile,
		"fileName":        filename,
		"caption":         caption,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) AnswerWithLocation(nameLocation string, address string, latitude float64, longitude float64) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendLocation(map[string]interface{}{
		"chatId":          chatId,
		"nameLocation":    nameLocation,
		"address":         address,
		"latitude":        latitude,
		"longitude":       longitude,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) AnswerWithPoll(message string, multipleAnswers bool, options []map[string]interface{}) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendPoll(map[string]interface{}{
		"chatId":          chatId,
		"message":         message,
		"options":         options,
		"multipleAnswers": multipleAnswers,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) AnswerWithContact(contact map[string]interface{}) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)
	response, err := n.GreenAPI.Methods().Sending().SendContact(map[string]interface{}{
		"chatId":          chatId,
		"contact":         contact,
		"quotedMessageId": idMessage,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendText(text string) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendMessage(map[string]interface{}{
		"chatId":  chatId,
		"message": text,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendUploadFile(filePath string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendFileByUpload(filePath, map[string]interface{}{
		"chatId":  chatId,
		"caption": caption,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendUrlFile(urlFile string, filename string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendFileByUrl(map[string]interface{}{
		"chatId":   chatId,
		"urlFile":  urlFile,
		"fileName": filename,
		"caption":  caption,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendLocation(nameLocation string, address string, latitude float64, longitude float64) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendLocation(map[string]interface{}{
		"chatId":       chatId,
		"nameLocation": nameLocation,
		"address":      address,
		"latitude":     latitude,
		"longitude":    longitude,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendPoll(message string, multipleAnswers bool, options []map[string]interface{}) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendPoll(map[string]interface{}{
		"chatId":          chatId,
		"message":         message,
		"options":         options,
		"multipleAnswers": multipleAnswers,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func (n *Notification) SendContact(contact map[string]interface{}) map[string]interface{} {
	chatId := tryParseChatId(n)
	response, err := n.GreenAPI.Methods().Sending().SendContact(map[string]interface{}{
		"chatId":  chatId,
		"contact": contact,
	})

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}

	return response
}

func tryParseChatId(n *Notification) string {
	var chatId string

	if n.Body["senderData"] != nil {
		chatId = n.Body["senderData"].(map[string]interface{})["chatId"].(string)
	} else {
		chatId = n.Body["from"].(string)
	}

	return chatId
}

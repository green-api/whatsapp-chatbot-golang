package whatsapp_chatbot_golang

func (n *Notification) AnswerWithText(text string) map[string]interface{} {
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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
	chatId := n.Body["senderData"].(map[string]interface{})["chatId"].(string)
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

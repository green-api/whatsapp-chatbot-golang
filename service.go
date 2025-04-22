package whatsapp_chatbot_golang

import (
	"encoding/json"
	greenapi "github.com/green-api/whatsapp-api-client-golang-v2"
	"path/filepath"
)

func (n *Notification) AnswerWithText(text string, linkPreview ...string) map[string]interface{} {
	_linkPreview := true
	if len(linkPreview) > 0 && linkPreview[0] == "false" {
		_linkPreview = false
	}

	chatId := tryParseChatId(n)

	idMessage := n.Body["idMessage"].(string)

	options := []greenapi.SendMessageOption{
		greenapi.OptionalQuotedMessageId(idMessage),
		greenapi.OptionalLinkPreview(_linkPreview),
	}
	resp, err := n.Sending().SendMessage(chatId, text, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) AnswerWithUploadFile(filePath string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)

	idMessage := n.Body["idMessage"].(string)
	fileName := filepath.Base(filePath)

	options := []greenapi.SendFileByUploadOption{
		greenapi.OptionalQuotedMessageIdSendUpload(idMessage),
	}
	if caption != "" {
		options = append(options, greenapi.OptionalCaptionSendUpload(caption))
	}
	resp, err := n.Sending().SendFileByUpload(chatId, filePath, fileName, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) AnswerWithUrlFile(urlFile string, filename string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)

	options := []greenapi.SendFileByUrlOption{
		greenapi.OptionalQuotedMessageIdSendUrl(idMessage),
	}
	if caption != "" {
		options = append(options, greenapi.OptionalCaptionSendUrl(caption))
	}
	resp, err := n.Sending().SendFileByUrl(chatId, urlFile, filename, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) AnswerWithLocation(nameLocation string, address string, latitude float64, longitude float64) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)

	options := []greenapi.SendLocationOption{
		greenapi.OptionalQuotedMessageIdLocation(idMessage),
	}
	if nameLocation != "" {
		options = append(options, greenapi.OptionalNameLocation(nameLocation))
	}
	if address != "" {
		options = append(options, greenapi.OptionalAddress(address))
	}
	resp, err := n.Sending().SendLocation(chatId, float32(latitude), float32(longitude), options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) AnswerWithPoll(message string, multipleAnswers bool, optionsStr []string) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)

	options := []greenapi.SendPollOption{
		greenapi.OptionalPollQuotedMessageId(idMessage),
		greenapi.OptionalMultipleAnswers(multipleAnswers),
	}
	resp, err := n.Sending().SendPoll(chatId, message, optionsStr, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) AnswerWithContact(contactData greenapi.Contact) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage := n.Body["idMessage"].(string)

	options := []greenapi.SendContactOption{
		greenapi.OptionalQuotedMessageIdContact(idMessage),
	}
	resp, err := n.Sending().SendContact(chatId, contactData, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendText(text string, linkPreview ...string) map[string]interface{} {
	_linkPreview := true
	if len(linkPreview) > 0 && linkPreview[0] == "false" {
		_linkPreview = false
	}
	chatId := tryParseChatId(n)

	options := []greenapi.SendMessageOption{
		greenapi.OptionalLinkPreview(_linkPreview),
	}
	resp, err := n.Sending().SendMessage(chatId, text, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendUploadFile(filePath string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	fileName := filepath.Base(filePath)

	var options []greenapi.SendFileByUploadOption
	if caption != "" {
		options = append(options, greenapi.OptionalCaptionSendUpload(caption))
	}
	resp, err := n.Sending().SendFileByUpload(chatId, filePath, fileName, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendUrlFile(urlFile string, filename string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)

	var options []greenapi.SendFileByUrlOption
	if caption != "" {
		options = append(options, greenapi.OptionalCaptionSendUrl(caption))
	}
	resp, err := n.Sending().SendFileByUrl(chatId, urlFile, filename, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendLocation(nameLocation string, address string, latitude float64, longitude float64) map[string]interface{} {
	chatId := tryParseChatId(n)

	var options []greenapi.SendLocationOption
	if nameLocation != "" {
		options = append(options, greenapi.OptionalNameLocation(nameLocation))
	}
	if address != "" {
		options = append(options, greenapi.OptionalAddress(address))
	}
	resp, err := n.Sending().SendLocation(chatId, float32(latitude), float32(longitude), options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendPoll(message string, multipleAnswers bool, optionsStr []string) map[string]interface{} {
	chatId := tryParseChatId(n)

	options := []greenapi.SendPollOption{
		greenapi.OptionalMultipleAnswers(multipleAnswers),
	}
	resp, err := n.Sending().SendPoll(chatId, message, optionsStr, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

func (n *Notification) SendContact(contactData greenapi.Contact) map[string]interface{} {
	chatId := tryParseChatId(n)

	resp, err := n.Sending().SendContact(chatId, contactData)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
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

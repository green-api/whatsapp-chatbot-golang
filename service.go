package whatsapp_chatbot_golang

import (
	"encoding/json"
	"path/filepath"

	greenapi "github.com/green-api/whatsapp-api-client-golang-v2"
)

func (n *Notification) AnswerWithText(text string, linkPreview ...string) map[string]interface{} {
	_linkPreview := true
	if len(linkPreview) > 0 && linkPreview[0] == "false" {
		_linkPreview = false
	}

	chatId := tryParseChatId(n)

	idMessage := n.Body["idMessage"].(string)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendMessageOption{
		greenapi.OptionalQuotedMessageId(idMessage),
		greenapi.OptionalLinkPreview(_linkPreview),
		greenapi.OptionalMessageTypingTime(typingTime),
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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendFileByUploadOption{
		greenapi.OptionalQuotedMessageIdSendUpload(idMessage),
		greenapi.OptionalUploadTypingTime(typingTime),
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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendFileByUrlOption{
		greenapi.OptionalQuotedMessageIdSendUrl(idMessage),
		greenapi.OptionalUrlTypingTime(typingTime),
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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendLocationOption{
		greenapi.OptionalQuotedMessageIdLocation(idMessage),
		greenapi.OptionalLocationTypingTime(typingTime),
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

	// idMessage, _ := n.Body["idMessage"].(string)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendPollOption{
		greenapi.OptionalMultipleAnswers(multipleAnswers),
		greenapi.OptionalPollTypingTime(typingTime),
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

	idMessage, _ := n.Body["idMessage"].(string)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendContactOption{
		greenapi.OptionalContactTypingTime(typingTime),
	}

	if idMessage != "" {
		options = append(options, greenapi.OptionalQuotedMessageIdContact(idMessage))
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

func (n *Notification) SendButtons(chatId string, body string, buttons []greenapi.InteractiveReplyButton) map[string]interface{} {

	resp, err := n.Sending().SendInteractiveButtonsReply(chatId, body, buttons)

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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendMessageOption{
		greenapi.OptionalLinkPreview(_linkPreview),
		greenapi.OptionalMessageTypingTime(typingTime),
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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendFileByUploadOption{
		greenapi.OptionalUploadTypingTime(typingTime),
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

func (n *Notification) SendUrlFile(urlFile string, filename string, caption string) map[string]interface{} {
	chatId := tryParseChatId(n)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	var options []greenapi.SendFileByUrlOption
	if typingTime != 0 {
		options = append(options, greenapi.OptionalUrlTypingTime(typingTime))
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

func (n *Notification) SendLocation(nameLocation string, address string, latitude float64, longitude float64) map[string]interface{} {
	chatId := tryParseChatId(n)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	var options []greenapi.SendLocationOption
	if typingTime != 0 {
		options = append(options, greenapi.OptionalLocationTypingTime(typingTime))
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

func (n *Notification) SendPoll(message string, multipleAnswers bool, optionsStr []string) map[string]interface{} {
	chatId := tryParseChatId(n)
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendPollOption{
		greenapi.OptionalMultipleAnswers(multipleAnswers),
		greenapi.OptionalPollTypingTime(typingTime),
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
	typingTime := 1000
	if val, ok := n.Body["typingTime"].(int); ok {
		typingTime = val
	}

	options := []greenapi.SendContactOption{
		greenapi.OptionalContactTypingTime(typingTime),
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

func (n *Notification) AnswerWithButtons(body string, buttons []greenapi.InteractiveReplyButton) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage, _ := n.Body["idMessage"].(string)

	options := []greenapi.SendInteractiveButtonsReplyOption{
		greenapi.OptionalInteractiveReplyQuotedMessageId(idMessage),
	}

	resp, err := n.Sending().SendInteractiveButtonsReply(chatId, body, buttons, options...)

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

func (n *Notification) AnswerWithInteractiveButtons(body string, buttons []greenapi.InteractiveButton, header string, footer string) map[string]interface{} {
	chatId := tryParseChatId(n)
	idMessage, _ := n.Body["idMessage"].(string)

	options := []greenapi.SendInteractiveButtonsOption{
		greenapi.OptionalInteractiveQuotedMessageId(idMessage),
	}
	if header != "" {
		options = append(options, greenapi.OptionalInteractiveHeader(header))
	}
	if footer != "" {

		options = append(options, greenapi.OptionalInteractiveFooter(footer))
	}

	resp, err := n.Sending().SendInteractiveButtons(chatId, body, buttons, options...)

	if err != nil {
		*n.ErrorChannel <- err
		return map[string]interface{}{"error": err}
	}
	var result map[string]interface{}
	_ = json.Unmarshal(resp.Body, &result)
	return result
}

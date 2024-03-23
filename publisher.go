package whatsapp_chatbot_golang

type Publisher struct {
	incomingMessage       []func(notification *Notification)
	outgoingMessage       []func(notification *Notification)
	outgoingMessageStatus []func(notification *Notification)
	incomingBlock         []func(notification *Notification)
	incomingCall          []func(notification *Notification)
	stateInstanceChanged  []func(notification *Notification)
	deviceInfo            []func(notification *Notification)
}

func (p *Publisher) publish(notification *Notification) {

	if notification.isIncomingMessage() {
		for _, value := range p.incomingMessage {
			value(notification)
		}

	} else if notification.isOutgoingMessage() {
		for _, value := range p.outgoingMessage {
			value(notification)
		}

	} else if notification.isOutgoingMessageStatus() {
		for _, value := range p.outgoingMessageStatus {
			value(notification)
		}

	} else if notification.isIncomingCall() {
		for _, value := range p.incomingCall {
			value(notification)
		}

	} else if notification.isIncomingBlock() {
		for _, value := range p.incomingBlock {
			value(notification)
		}

	} else if notification.isStateInstanceChanged() {
		for _, value := range p.stateInstanceChanged {
			value(notification)
		}

	} else if notification.isDeviceInfo() {
		for _, value := range p.deviceInfo {
			value(notification)
		}
	}
}

func (p *Publisher) IncomingMessageHandler(f func(notification *Notification)) {
	p.incomingMessage = append(p.incomingMessage, f)
}

func (p *Publisher) OutgoingMessageHandler(f func(notification *Notification)) {
	p.outgoingMessage = append(p.outgoingMessage, f)
}

func (p *Publisher) OutgoingMessageStatusHandler(f func(notification *Notification)) {
	p.outgoingMessageStatus = append(p.outgoingMessageStatus, f)
}

func (p *Publisher) IncomingCallHandler(f func(notification *Notification)) {
	p.incomingCall = append(p.outgoingMessageStatus, f)
}

func (p *Publisher) IncomingBlockHandler(f func(notification *Notification)) {
	p.incomingBlock = append(p.outgoingMessageStatus, f)
}

func (p *Publisher) StateInstanceChangedHandler(f func(notification *Notification)) {
	p.stateInstanceChanged = append(p.outgoingMessageStatus, f)
}

func (p *Publisher) DeviceInfoHandler(f func(notification *Notification)) {
	p.deviceInfo = append(p.outgoingMessageStatus, f)
}

func (p *Publisher) clearAll() {
	p.incomingMessage = p.incomingMessage[:0]
	p.outgoingMessage = p.outgoingMessage[:0]
	p.outgoingMessageStatus = p.outgoingMessageStatus[:0]
	p.incomingBlock = p.incomingBlock[:0]
	p.incomingCall = p.incomingCall[:0]
	p.stateInstanceChanged = p.stateInstanceChanged[:0]
	p.deviceInfo = p.deviceInfo[:0]
}

package messenger

// Inter-node communication.

type Messenger struct {
	sender MessageSender
}

func NewMessenger(sender MessageSender) *Messenger {
	return &Messenger{sender: sender}
}


func (m *Messenger) SendMessage(msg *Message) error {
	return m.sender.SendMessage(msg)
}
package messenger


type MessageSender interface {
	SendMessage(msg *Message) error
}
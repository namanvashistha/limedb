package messenger

type Message struct {
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
	Sender  string `json:"sender"`
	DestinationNodeURL string `json:"destinationNodeURL"`
}

func NewMessage(msgType string, payload []byte, sender string, destNodeURL string) *Message {
	return &Message{
		Type:    msgType,
		Payload: payload,
		Sender:  sender,
		DestinationNodeURL: destNodeURL,
	}
}
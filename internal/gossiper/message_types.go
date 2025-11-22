package gossiper

type Digests struct {
	NodeURL   string `json:"nodeUrl"`
	Heartbeat int    `json:"heartbeat"`
}

type SynPayload struct {
	Digests []Digests `json:"digests"`
}

type AckPayload struct {
	DigestsUpdate  []Digests `json:"digestsUpdate"`
	DigestsRequest []Digests `json:"digestsRequest"`
}

type Ack2Payload struct {
	DigestsUpdate []Digests `json:"digestsUpdate"`
}

// GossipMessage wraps all gossip message types
type GossipMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

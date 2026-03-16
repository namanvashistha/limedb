package gossiper

// EndpointDigest is what a node sends about one endpoint in a SYN.
// Matches Cassandra's (endpoint, generation, maxVersion) tuple.
//   - Generation: Unix epoch seconds when that node last started. Increments on
//     restart, so a higher generation always wins regardless of version.
//   - Version:    Monotonically increasing counter, bumped once per gossip round
//     by that node for itself. Only meaningful compared to a
//     previous observation of the same endpoint.
type EndpointDigest struct {
	NodeURL    string `json:"nodeUrl"`
	Generation int64  `json:"generation"` // Unix epoch seconds at node boot
	Version    int    `json:"version"`    // gossip round counter (owned by that node)
}

// SynPayload is the initial probe: "here is what I know about everyone."
type SynPayload struct {
	Digests []EndpointDigest `json:"digests"`
}

// AckPayload is the response to a SYN.
//   - Updates:  endpoints where the responder has fresher state → send to initiator
//   - Requests: endpoints where the initiator appears fresher → please send me those
type AckPayload struct {
	Updates  []EndpointDigest `json:"updates"`
	Requests []EndpointDigest `json:"requests"`
}

// Ack2Payload carries the state requested in the ACK's Requests list.
type Ack2Payload struct {
	Updates []EndpointDigest `json:"updates"`
}

// GossipMessage is the outer envelope sent over HTTP.
type GossipMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

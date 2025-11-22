package messenger

import (
	"encoding/json"
	"fmt"
	"limedb/internal/logger"
	"time"

	"github.com/valyala/fasthttp"
)

type FastHTTPMessageSender struct {
	client *fasthttp.Client
}

func NewFasthttpMessengeSender(client *fasthttp.Client) *FastHTTPMessageSender {
	return &FastHTTPMessageSender{client: client}
}

func (s *FastHTTPMessageSender) SendMessage(msg *Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(msg.DestinationNodeURL)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(b)

	if err := s.client.DoTimeout(req, resp, 2*time.Second); err != nil {
		return err
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		logger.Error("unexpected status code: %d", resp.StatusCode())
		return fmt.Errorf("unexpected status code",
			"statusCode", resp.StatusCode(), "peer", msg.DestinationNodeURL
		)
	}
	return nil
}

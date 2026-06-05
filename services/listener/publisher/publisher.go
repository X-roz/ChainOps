package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"listener/schema"

	"github.com/nats-io/nats.go"
)

const batchSize = 100

var pubLog = slog.With("publisher", "[nats]")

// Publisher holds an active NATS JetStream connection and the target subject.
type Publisher struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	subject string
}

// New connects to the NATS server at natsURL and returns a ready Publisher.
// The connection is configured for unlimited reconnects so transient network
// blips are handled transparently.
func New(natsURL, subject string) (*Publisher, error) {
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			pubLog.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			pubLog.Info("NATS reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS at %s: %w", natsURL, err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("JetStream context: %w", err)
	}

	pubLog.Info("NATS publisher ready", "subject", subject)
	return &Publisher{nc: nc, js: js, subject: subject}, nil
}

func (p *Publisher) Publish(ctx context.Context, msg schema.BlockActivityMessage) error {
	batches := chunk(msg, batchSize)
	total := len(batches)
	for i, batch := range batches {
		if err := p.publishBatch(ctx, batch, i+1, total); err != nil {
			return fmt.Errorf("block %d batch %d/%d: %w", msg.BlockNumber, i+1, total, err)
		}
	}
	return nil
}

func (p *Publisher) Close() {
	if err := p.nc.Drain(); err != nil {
		pubLog.Error("failed to drain NATS connection", "error", err)
	}
}

func (p *Publisher) publishBatch(ctx context.Context, msg schema.BlockActivityMessage, batchIdx, totalBatches int) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	natsMsg := &nats.Msg{
		Subject: p.subject,
		Data:    data,
		Header:  nats.Header{},
	}
	natsMsg.Header.Set("X-Network-ID", msg.NetworkID)
	natsMsg.Header.Set("X-Block-Number", strconv.FormatUint(msg.BlockNumber, 10))
	natsMsg.Header.Set("X-Batch-Index", strconv.Itoa(batchIdx))
	natsMsg.Header.Set("X-Total-Batches", strconv.Itoa(totalBatches))

	ack, err := p.js.PublishMsg(natsMsg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("JetStream publish: %w", err)
	}

	pubLog.Info("published",
		"block", msg.BlockNumber,
		"batch", fmt.Sprintf("%d/%d", batchIdx, totalBatches),
		"events", len(msg.Events),
		"stream", ack.Stream,
		"seq", ack.Sequence,
	)
	return nil
}

func chunk(msg schema.BlockActivityMessage, n int) []schema.BlockActivityMessage {
	if len(msg.Events) <= n {
		return []schema.BlockActivityMessage{msg}
	}

	var batches []schema.BlockActivityMessage
	for i := 0; i < len(msg.Events); i += n {
		end := i + n
		if end > len(msg.Events) {
			end = len(msg.Events)
		}
		batches = append(batches, schema.BlockActivityMessage{
			NetworkID:      msg.NetworkID,
			BlockNumber:    msg.BlockNumber,
			BlockHash:      msg.BlockHash,
			BlockTimestamp: msg.BlockTimestamp,
			Events:         msg.Events[i:end],
		})
	}
	return batches
}

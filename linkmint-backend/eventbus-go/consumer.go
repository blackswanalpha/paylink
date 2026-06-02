package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

// HandleFunc processes one event: its logical name and canonical payload bytes. Returning an error
// means "not handled" — the offset is NOT committed and the event is redelivered (at-least-once), so
// handlers MUST be idempotent. A nil return makes the event committable.
type HandleFunc func(ctx context.Context, name string, payload json.RawMessage) error

// Consumer is a Kafka consumer-group member that decodes envelopes and dispatches them to a
// HandleFunc, committing offsets only after a successful handle.
type Consumer struct {
	client *kgo.Client
	log    *slog.Logger
}

// NewConsumer joins the configured consumer group, subscribed to the given domain topics.
func NewConsumer(cfg Config, topics []string, log *slog.Logger) (*Consumer, error) {
	if log == nil {
		log = slog.Default()
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("eventbus: no kafka brokers configured")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("eventbus: consumer group id is required")
	}
	if len(topics) == 0 {
		return nil, fmt.Errorf("eventbus: at least one topic is required")
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		// A group with no committed offset starts at the earliest record, so a newly-added consumer
		// never silently skips events produced before it joined (at-least-once; idempotent handlers
		// absorb any replay). Once the group commits, it resumes from its committed offset.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("eventbus: new consumer: %w", err)
	}
	return &Consumer{client: cl, log: log}, nil
}

// Run polls and dispatches until ctx is cancelled, then closes the client. Within a partition it
// processes records in offset order and commits the clean prefix; the first handle error stops that
// partition for the round (the failed record + everything after it redeliver). A record whose bytes
// fail to decode is logged and skipped+committed (poison-safe) — it can never succeed and would
// otherwise block the partition forever.
func (c *Consumer) Run(ctx context.Context, handle HandleFunc) error {
	defer c.client.Close()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fetches := c.client.PollRecords(ctx, 256)
		if fetches.IsClientClosed() {
			return nil
		}
		fetches.EachError(func(t string, p int32, err error) {
			if !errors.Is(err, context.Canceled) {
				c.log.Warn("eventbus_fetch_error", "topic", t, "partition", p, "err", err.Error())
			}
		})
		var commit []*kgo.Record
		fetches.EachPartition(func(ftp kgo.FetchTopicPartition) {
			for _, rec := range ftp.Records {
				env, err := UnmarshalEnvelope(rec.Value)
				if err != nil {
					c.log.Warn("eventbus_decode_failed", "topic", rec.Topic, "offset", rec.Offset, "err", err.Error())
					commit = append(commit, rec) // poison-safe: skip + commit
					continue
				}
				// Continue the producer's trace under a consume span; the handler runs within it.
				recCtx, span := startConsumeSpan(ctx, env.Name, rec.Headers)
				herr := handle(recCtx, env.Name, env.Payload)
				span.End()
				if herr != nil {
					c.log.Warn("eventbus_handle_failed", "name", env.Name, "key", env.Key, "offset", rec.Offset, "err", herr.Error())
					return // stop this partition; do not commit the failed record → it redelivers
				}
				commit = append(commit, rec)
			}
		})
		if len(commit) > 0 {
			if err := c.client.CommitRecords(ctx, commit...); err != nil && !errors.Is(err, context.Canceled) {
				c.log.Warn("eventbus_commit_failed", "err", err.Error())
			}
		}
	}
}

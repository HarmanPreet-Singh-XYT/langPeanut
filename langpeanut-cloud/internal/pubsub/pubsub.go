// Package pubsub provides durable GitHub webhook intake via Google Cloud
// Pub/Sub. GitHub delivers webhooks best-effort with a short retry window;
// publishing each verified delivery to a topic before processing it means a
// deploy, restart, or transient DB hiccup can't silently drop a push or PR
// comment event — the message sits in the subscription until it's acked.
//
// When GCP_PROJECT_ID is unset (local dev, tests), Publisher is nil and
// callers fall back to processing webhooks synchronously in the HTTP
// handler. This package never blocks that fallback path on GCP credentials.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/pubsub"
)

// WebhookEvent is the payload published for each verified GitHub webhook
// delivery. It carries everything the original handleWebhook switch needs,
// so the subscriber can run the exact same processing logic the HTTP path
// used to run inline.
type WebhookEvent struct {
	EventType string `json:"event_type"` // X-GitHub-Event header, e.g. "push"
	Delivery  string `json:"delivery"`   // X-GitHub-Delivery header, for dedup/tracing
	Body      []byte `json:"body"`       // raw JSON webhook payload
}

// Publisher publishes verified webhook deliveries to a Pub/Sub topic.
type Publisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// NewPublisher connects to the given GCP project and topic. The topic must
// already exist (created out-of-band via `gcloud pubsub topics create` or
// Terraform) — this package intentionally doesn't auto-create infrastructure.
func NewPublisher(ctx context.Context, projectID, topicID string) (*Publisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client: %w", err)
	}
	topic := client.Topic(topicID)
	return &Publisher{client: client, topic: topic}, nil
}

// Publish sends a webhook event to the topic and waits for the publish
// to be acknowledged by the Pub/Sub service (not by a downstream subscriber).
func (p *Publisher) Publish(ctx context.Context, evt WebhookEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}
	result := p.topic.Publish(ctx, &pubsub.Message{Data: data})
	if _, err := result.Get(ctx); err != nil {
		return fmt.Errorf("publish webhook event: %w", err)
	}
	return nil
}

// Close flushes any pending publishes and releases the client.
func (p *Publisher) Close() error {
	p.topic.Stop()
	return p.client.Close()
}

// Handler processes one decoded WebhookEvent. Implemented by
// internal/api.Handler.ProcessWebhookEvent; kept as an interface here so
// this package doesn't import internal/api (which would create an import
// cycle, since internal/api imports this package to publish).
type Handler interface {
	ProcessWebhookEvent(ctx context.Context, evt WebhookEvent) error
}

// RunSubscriber pulls webhook events from the subscription and hands each
// one to handler.ProcessWebhookEvent. It blocks until ctx is cancelled, and
// is meant to be started as a goroutine alongside the worker poll loop.
//
// Messages are acked only on successful processing; a processing error nacks
// the message so Pub/Sub redelivers it (subject to the subscription's own
// retry/dead-letter policy configured in GCP).
func RunSubscriber(ctx context.Context, projectID, subscriptionID string, handler Handler) error {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("pubsub client: %w", err)
	}
	defer client.Close()

	sub := client.Subscription(subscriptionID)

	slog.Info("pubsub: webhook subscriber starting", "subscription", subscriptionID)
	err = sub.Receive(ctx, func(msgCtx context.Context, msg *pubsub.Message) {
		var evt WebhookEvent
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			slog.Error("pubsub: undecodable webhook message, dropping", "err", err)
			msg.Ack() // never redeliver a message that can't be parsed
			return
		}

		procCtx, cancel := context.WithTimeout(msgCtx, 30*time.Second)
		defer cancel()

		if err := handler.ProcessWebhookEvent(procCtx, evt); err != nil {
			slog.Error("pubsub: webhook processing failed, nacking for redelivery",
				"event_type", evt.EventType, "delivery", evt.Delivery, "err", err)
			msg.Nack()
			return
		}
		msg.Ack()
	})
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("subscription receive: %w", err)
	}
	return nil
}

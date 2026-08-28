package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

type DeliveryStore interface {
	ClaimDelivery(context.Context, time.Duration) (*domainwebhook.Delivery, *domainwebhook.Event, error)
	MarkDeliveryDelivered(context.Context, string, int, time.Time) error
	MarkDeliveryFailed(context.Context, string, string, *int, bool, int, time.Duration, time.Duration, time.Time) error
}

type Logger interface {
	Info(string, ...any)
	Error(string, ...any)
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Dispatcher struct {
	store        DeliveryStore
	cipher       *domainwebhook.Cipher
	client       HTTPClient
	logger       Logger
	pollInterval time.Duration
	lockTimeout  time.Duration
	maxAttempts  int
	backoffBase  time.Duration
	backoffMax   time.Duration
	now          func() time.Time
}

type DispatcherConfig struct {
	PollInterval time.Duration
	LockTimeout  time.Duration
	MaxAttempts  int
	BackoffBase  time.Duration
	BackoffMax   time.Duration
}

func NewDispatcher(store DeliveryStore, cipher *domainwebhook.Cipher, client HTTPClient, logger Logger, cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{store: store, cipher: cipher, client: client, logger: logger,
		pollInterval: cfg.PollInterval, lockTimeout: cfg.LockTimeout, maxAttempts: cfg.MaxAttempts,
		backoffBase: cfg.BackoffBase, backoffMax: cfg.BackoffMax, now: time.Now}
}

func (d *Dispatcher) Run(ctx context.Context) {
	timer := time.NewTicker(d.pollInterval)
	defer timer.Stop()
	for {
		processed, err := d.DispatchOne(ctx)
		if err != nil && d.logger != nil {
			d.logger.Error("webhook dispatch cycle failed", "error_category", "dispatch")
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (d *Dispatcher) DispatchOne(ctx context.Context) (bool, error) {
	delivery, event, err := d.store.ClaimDelivery(ctx, d.lockTimeout)
	if err != nil || delivery == nil {
		return false, err
	}
	config, err := d.cipher.Decrypt(delivery.EncryptedConfig, delivery.FormID)
	if err != nil {
		category := "configuration"
		return true, errors.Join(err, d.store.MarkDeliveryFailed(ctx, delivery.ID, category, nil, false,
			d.maxAttempts, d.backoffBase, d.backoffMax, d.now().UTC()))
	}
	body, err := json.Marshal(event)
	if err != nil {
		return true, errors.Join(err, d.store.MarkDeliveryFailed(ctx, delivery.ID, "payload", nil, false,
			d.maxAttempts, d.backoffBase, d.backoffMax, d.now().UTC()))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.DestinationURL, bytes.NewReader(body))
	if err != nil {
		return true, errors.Join(err, d.store.MarkDeliveryFailed(ctx, delivery.ID, "destination", nil, false,
			d.maxAttempts, d.backoffBase, d.backoffMax, d.now().UTC()))
	}
	for name, value := range config.Headers {
		request.Header.Set(name, value)
	}
	now := d.now().UTC()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "GoFormX-Webhook/1.0")
	request.Header.Set(HeaderDeliveryID, delivery.ID)
	request.Header.Set(HeaderTimestamp, timestamp)
	request.Header.Set(HeaderSignature, Sign(config.SigningSecret, delivery.ID, timestamp, body))

	response, err := d.client.Do(request)
	if err != nil {
		markErr := d.store.MarkDeliveryFailed(ctx, delivery.ID, "network", nil, true,
			d.maxAttempts, d.backoffBase, d.backoffMax, now)
		return true, errors.Join(err, markErr)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if err := d.store.MarkDeliveryDelivered(ctx, delivery.ID, response.StatusCode, now); err != nil {
			return true, err
		}
		if d.logger != nil {
			d.logger.Info("webhook delivered", "delivery_id", delivery.ID, "form_id", delivery.FormID,
				"attempt", delivery.AttemptCount, "http_status", response.StatusCode)
		}
		return true, nil
	}
	retryable := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= 500
	category := "http_4xx"
	if retryable {
		category = "http_retryable"
	}
	return true, d.store.MarkDeliveryFailed(ctx, delivery.ID, category, &response.StatusCode, retryable,
		d.maxAttempts, d.backoffBase, d.backoffMax, now)
}

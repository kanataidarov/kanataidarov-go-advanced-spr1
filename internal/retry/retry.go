package retry

import (
	"context"
	"fmt"
	"time"
)

var Backoff = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

type Policy struct {
	Delays    []time.Duration
	Retriable func(error) bool
	OnRetry   func(err error, attempt int, delay time.Duration)
}

func New(retriable func(error) bool, onRetry func(err error, attempt int, delay time.Duration)) Policy {
	return Policy{Delays: Backoff, Retriable: retriable, OnRetry: onRetry}
}

func (p Policy) Do(ctx context.Context, op func(ctx context.Context) error) error {
	_, err := DoValue(ctx, p, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, op(ctx)
	})

	return err
}

func DoValue[T any](ctx context.Context, p Policy, op func(ctx context.Context) (T, error)) (T, error) {
	v, err := op(ctx)

	for i, delay := range p.Delays {
		if err == nil || p.Retriable == nil || !p.Retriable(err) {
			return v, err
		}

		if p.OnRetry != nil {
			p.OnRetry(err, i+1, delay)
		}

		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()

			return v, fmt.Errorf("retry aborted: %w: %w", ctx.Err(), err)
		case <-timer.C:
		}

		v, err = op(ctx)
	}

	if err != nil && p.Retriable != nil && p.Retriable(err) {
		return v, fmt.Errorf("gave up after %d attempts: %w", len(p.Delays)+1, err)
	}

	return v, err
}

package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var (
	errTemporary = errors.New("temporary")
	errPermanent = errors.New("permanent")
)

func isTemporary(err error) bool { return errors.Is(err, errTemporary) }

func testPolicy() (Policy, *[]time.Duration) {
	var waited []time.Duration

	p := Policy{
		Delays:    []time.Duration{time.Millisecond, 3 * time.Millisecond, 5 * time.Millisecond},
		Retriable: isTemporary,
		OnRetry: func(_ error, _ int, delay time.Duration) {
			waited = append(waited, delay)
		},
	}

	return p, &waited
}

func failing(results ...error) (func(context.Context) error, *int) {
	calls := 0

	return func(context.Context) error {
		calls++
		if calls <= len(results) {
			return results[calls-1]
		}

		return nil
	}, &calls
}

func TestDoSucceedsFirstTime(t *testing.T) {
	p, waited := testPolicy()
	op, calls := failing()

	if err := p.Do(t.Context(), op); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *calls != 1 || len(*waited) != 0 {
		t.Errorf("got %d calls and %d retries, want 1 and 0", *calls, len(*waited))
	}
}

func TestDoRetriesTemporaryErrors(t *testing.T) {
	p, waited := testPolicy()
	op, calls := failing(errTemporary, errTemporary)

	if err := p.Do(t.Context(), op); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *calls != 3 {
		t.Errorf("got %d calls, want 3", *calls)
	}

	if want := p.Delays[:2]; len(*waited) != 2 || (*waited)[0] != want[0] || (*waited)[1] != want[1] {
		t.Errorf("got delays %v, want %v", *waited, want)
	}
}

func TestDoStopsAfterThreeRetries(t *testing.T) {
	p, waited := testPolicy()
	op, calls := failing(errTemporary, errTemporary, errTemporary, errTemporary, errTemporary)

	err := p.Do(t.Context(), op)
	if !errors.Is(err, errTemporary) {
		t.Fatalf("got error %v, want wrapped %v", err, errTemporary)
	}

	if *calls != 4 {
		t.Errorf("got %d calls, want 1 + 3 retries", *calls)
	}

	if len(*waited) != 3 {
		t.Errorf("got %d retries, want 3", len(*waited))
	}
}

func TestDoDoesNotRetryPermanentErrors(t *testing.T) {
	p, waited := testPolicy()
	op, calls := failing(errPermanent)

	if err := p.Do(t.Context(), op); !errors.Is(err, errPermanent) {
		t.Fatalf("got error %v, want %v", err, errPermanent)
	}

	if *calls != 1 || len(*waited) != 0 {
		t.Errorf("got %d calls and %d retries, want 1 and 0", *calls, len(*waited))
	}
}

func TestDoAbortsOnContextCancel(t *testing.T) {
	p := Policy{Delays: []time.Duration{time.Hour}, Retriable: isTemporary}
	op, calls := failing(errTemporary, errTemporary)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := p.Do(ctx, op)

	if time.Since(start) > time.Second {
		t.Fatal("Do must not wait for the delay after the context is done")
	}

	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errTemporary) {
		t.Errorf("got error %v, want both the context error and the last error", err)
	}

	if *calls != 1 {
		t.Errorf("got %d calls, want 1", *calls)
	}
}

func TestBackoffMatchesTask(t *testing.T) {
	want := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}
	if len(Backoff) != len(want) {
		t.Fatalf("got %v, want %v", Backoff, want)
	}

	for i := range want {
		if Backoff[i] != want[i] {
			t.Errorf("got %v, want %v", Backoff, want)
		}
	}
}

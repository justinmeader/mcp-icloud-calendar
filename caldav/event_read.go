package caldav

import (
	"context"
	"fmt"
)

// GetEvent fetches the stored resource without a date-range query.
func (c *Client) GetEvent(ctx context.Context, path string) (*Event, error) {
	object, err := c.backend.GetCalendarObject(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	if object == nil || object.Data == nil {
		return nil, fmt.Errorf("empty calendar object")
	}
	return c.parseCalendarObject(object)
}
func readService(ctx context.Context, c CalendarService, path string) (*Event, error) {
	reader, ok := c.(EventReader)
	if !ok {
		return nil, fmt.Errorf("direct event read unavailable for this account")
	}
	return reader.GetEvent(ctx, path)
}
func (r *RateLimitedClient) GetEvent(ctx context.Context, path string) (*Event, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return readService(ctx, r.inner, path)
}
func (r *RetryClient) GetEvent(ctx context.Context, path string) (*Event, error) {
	var event *Event
	err := r.retry(ctx, "GetEvent", func() error { var e error; event, e = readService(ctx, r.inner, path); return e })
	return event, err
}

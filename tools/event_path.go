package tools

import (
	"fmt"
	"github.com/rgabriel/mcp-icloud-calendar/caldav"
	"net/url"
	"strings"
)

// resolveEventResource accepts a plain resource path scoped to its calendar.
func resolveEventResource(client caldav.CalendarService, calendar string, args map[string]interface{}) (string, error) {
	parsed, err := url.Parse(calendar)
	if err != nil || !strings.HasPrefix(calendar, "/") || strings.HasPrefix(calendar, "//") || strings.ContainsAny(calendar, "%?#\\") || parsed.Host != "" || parsed.Scheme != "" {
		return "", fmt.Errorf("invalid calendarId: expected a plain absolute CalDAV path")
	}
	if err := caldav.ValidateCalendarPath(calendar); err != nil {
		return "", fmt.Errorf("invalid calendarId: %w", err)
	}
	path, _ := args["eventPath"].(string)
	if path != "" {
		prefix := strings.TrimSuffix(calendar, "/") + "/"
		if !strings.HasPrefix(path, prefix) {
			return "", fmt.Errorf("eventPath must belong to calendarId")
		}
		resource := strings.TrimPrefix(path, prefix)
		if strings.ContainsAny(resource, "%?#\\") {
			return "", fmt.Errorf("invalid eventPath: encoded or query characters are unsupported")
		}
		if err := caldav.ValidateEventID(resource); err != nil {
			return "", fmt.Errorf("invalid eventPath: %w", err)
		}
		return path, nil
	}
	id, _ := args["eventId"].(string)
	if id == "" {
		return "", fmt.Errorf("eventPath or eventId is required")
	}
	if strings.ContainsAny(id, "%?#\\") {
		return "", fmt.Errorf("invalid eventId: encoded or query characters are unsupported")
	}
	if err := caldav.ValidateEventID(id); err != nil {
		return "", fmt.Errorf("invalid eventId: %w", err)
	}
	return client.GetEventPath(calendar, id), nil
}

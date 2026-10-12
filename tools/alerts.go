package tools

import (
	"encoding/json"
	"fmt"
	"github.com/rgabriel/mcp-icloud-calendar/caldav"
	"strings"
)

// parseAlerts distinguishes omission from [] (explicitly remove all alarms).
func parseAlerts(args map[string]interface{}) (*[]caldav.Alert, error) {
	raw, exists := args["alerts"]
	if !exists {
		return nil, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("alerts must be a JSON array string")
	}
	if !strings.HasPrefix(strings.TrimSpace(text), "[") {
		return nil, fmt.Errorf("alerts must be a JSON array; use [] to clear")
	}
	var a []caldav.Alert
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&a); err != nil {
		return nil, fmt.Errorf("invalid alerts JSON: %w", err)
	}
	// Unmarshal independently checks trailing data after the array.
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		return nil, fmt.Errorf("invalid alerts JSON: %w", err)
	}
	if err := caldav.ValidateAlerts(a); err != nil {
		return nil, err
	}
	return &a, nil
}

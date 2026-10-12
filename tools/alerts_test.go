package tools

import (
	"context"
	"github.com/rgabriel/mcp-icloud-calendar/caldav"
	"testing"
)

func TestEnhancementRejectInvalidAlerts(t *testing.T) {
	for _, alert := range []interface{}{`not-json`, `null`, `[{"action":"EMAIL","trigger":"-PT5M"}]`, `[{"action":"DISPLAY","trigger":"garbage"}]`, float64(4)} {
		result, err := CreateEventHandler(testAccounts(&caldav.MockClient{}, "/cal/default"))(context.Background(), newCreateRequest(map[string]interface{}{"title": "Fixture", "startTime": "2020-01-02T10:00:00Z", "endTime": "2020-01-02T11:00:00Z", "alerts": alert}))
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Errorf("invalid alerts accepted: %v", alert)
		}
	}
}

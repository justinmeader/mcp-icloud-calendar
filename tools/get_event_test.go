package tools

import (
	"context"
	"fmt"
	ext "github.com/emersion/go-webdav/caldav"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-calendar/caldav"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestEnhancementGetEventResourcePath(t *testing.T) {
	// Real CalDAV GET/parse path: UID deliberately differs from resource filename.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cal/resource.ics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		fmt.Fprint(w, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//fixture//EN\r\nBEGIN:VEVENT\r\nUID:different-uid\r\nDTSTAMP:20200101T000000Z\r\nDTSTART:20200102T100000Z\r\nDTEND:20200102T110000Z\r\nSUMMARY:Read fixture\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	}))
	defer ts.Close()
	transport, err := ext.NewClient(ts.Client(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := caldav.NewRetryClient(caldav.NewRateLimitedClient(caldav.NewClientWithBackend(transport), 100, 10), 0, 0)
	handler := GetEventHandler(testAccounts(client, "/cal/"))
	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]interface{}{"eventPath": "/cal/resource.ics"}}})
	if err != nil || result.IsError {
		t.Fatalf("read failed: %v %v", err, result)
	}
	if !strings.Contains(result.Content[0].(mcp.TextContent).Text, `"id": "different-uid"`) {
		t.Fatal("resource lookup failed")
	}
	for _, path := range []string{"/other/resource.ics", "/cal/../resource.ics", "/cal/resource.ics?x=1", "/cal/%2e%2e%2fresource.ics"} {
		result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]interface{}{"eventPath": path}}})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Errorf("unsafe path accepted: %s", path)
		} else {
			message := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(message, "invalid eventPath") && !strings.Contains(message, "must belong") {
				t.Errorf("unsafe path reached transport instead of validation: %s: %s", path, message)
			}
		}
	}
}
func TestEnhancementUpdateByResourcePath(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//fixture//EN\r\nBEGIN:VEVENT\r\nUID:different-uid\r\nDTSTAMP:20200101T000000Z\r\nDTSTART:20200102T100000Z\r\nDTEND:20200102T110000Z\r\nSUMMARY:Before\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	var mu sync.Mutex
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cal/resource.ics" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "PUT" {
			body, _ := io.ReadAll(r.Body)
			data = string(body)
			w.WriteHeader(201)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		fmt.Fprint(w, data)
	}))
	defer ts.Close()
	transport, _ := ext.NewClient(ts.Client(), ts.URL)
	client := caldav.NewClientWithBackend(transport)
	accounts := testAccounts(client, "/cal/")
	result, err := UpdateEventHandler(accounts)(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]interface{}{"eventPath": "/cal/resource.ics", "alerts": `[{"action":"DISPLAY","trigger":"-PT10M"}]`}}})
	if err != nil || result.IsError {
		t.Fatalf("resource-path update failed: %v %v", err, result)
	}
	result, err = GetEventHandler(accounts)(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]interface{}{"eventPath": "/cal/resource.ics"}}})
	if err != nil || result.IsError || !strings.Contains(result.Content[0].(mcp.TextContent).Text, `"trigger": "-PT10M"`) {
		t.Fatal("path edit did not persist reminder")
	}
}

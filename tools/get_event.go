package tools

import (
	"context"
	"encoding/json"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-calendar/caldav"
)

// GetEventHandler reads one calendar resource; a returned path is preferred to UID.
func GetEventHandler(accounts *AccountClients) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		account, _ := args["account"].(string)
		client, defaultCalendar, err := accounts.Resolve(account)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		calendar, _ := args["calendarId"].(string)
		if calendar == "" {
			calendar = defaultCalendar
		}
		if calendar == "" {
			return mcp.NewToolResultError("calendarId is required"), nil
		}
		path, err := resolveEventResource(client, calendar, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		reader, ok := client.(caldav.EventReader)
		if !ok {
			return mcp.NewToolResultError("direct event read unavailable for this account"), nil
		}
		event, err := reader.GetEvent(ctx, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(raw)), nil
	}
}

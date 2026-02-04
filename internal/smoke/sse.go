package smoke

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SSEEvent struct {
	Event string
	Data  json.RawMessage
}

// WaitForEvents connects to an SSE endpoint and returns once predicate matches.
func WaitForEvents(ctx context.Context, client *http.Client, url string, predicate func(SSEEvent) bool) ([]SSEEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sse http %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	// Allow larger lines than default (64K).
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var events []SSEEvent
	var curEvent string
	var curData strings.Builder

	flush := func() bool {
		if curEvent == "" && curData.Len() == 0 {
			return false
		}
		ev := SSEEvent{Event: curEvent, Data: json.RawMessage(strings.TrimSpace(curData.String()))}
		events = append(events, ev)
		curEvent = ""
		curData.Reset()
		return predicate != nil && predicate(ev)
	}

	deadline, hasDeadline := ctx.Deadline()
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if flush() {
				return events, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			curEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if curData.Len() > 0 {
				curData.WriteByte('\n')
			}
			curData.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}

		if hasDeadline && time.Now().After(deadline) {
			return events, ctx.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return events, err
	}
	return events, ctx.Err()
}

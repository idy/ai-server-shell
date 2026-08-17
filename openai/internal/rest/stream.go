package rest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/idy/ai-server-shell/backend"
)

func writeSSE(ctx context.Context, writer http.ResponseWriter, requestID string, status int, stream backend.Stream) {
	defer stream.Close()
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeErrorStatus(writer, requestID, http.StatusInternalServerError, &backend.Error{Kind: backend.ErrorInternal, Message: "Streaming is not supported by this server."})
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(status)
	flusher.Flush()

	buffer := bufio.NewWriter(writer)
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-stream.Events():
			if !open {
				return
			}
			if event.ID != "" {
				_, _ = fmt.Fprintf(buffer, "id: %s\n", sanitizeSSE(event.ID))
			}
			if event.Type != "" {
				_, _ = fmt.Fprintf(buffer, "event: %s\n", sanitizeSSE(event.Type))
			}
			data := event.Data
			if len(data) == 0 {
				data, _ = json.Marshal(map[string]string{"type": event.Type})
			}
			for line := range strings.SplitSeq(string(data), "\n") {
				_, _ = fmt.Fprintf(buffer, "data: %s\n", line)
			}
			_, _ = buffer.WriteString("\n")
			if err := buffer.Flush(); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func sanitizeSSE(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ingestPayload is the real, exact wire shape POSTed to the backend's ingestion route.
type ingestPayload struct {
	InstallID string  `json:"install_id"`
	Events    []Event `json:"events"`
}

// Flush sends the batch in exactly one real HTTP call. Fail-safe by design: telemetry
// must never fail a real command or add noticeable latency to it -- errors are silently
// swallowed (there is deliberately no logging of telemetry failures to stderr, which
// would be noise for a background, best-effort concern the user didn't ask to watch),
// and a short, bounded timeout keeps a slow/unreachable telemetry endpoint from ever
// hanging the CLI itself.
func (b *Batch) Flush(apiBase string) {
	if b.level == LevelOff || b.IsEmpty() {
		return
	}
	events := b.Events()

	payload := ingestPayload{InstallID: installID(), Events: events}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/v1/telemetry/ingest", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

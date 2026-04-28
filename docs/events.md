# SSE Events

All run events are emitted on `GET /v1/runs/:id/events` as JSON payloads.

## Event Types

- `run.created`
- `run.planned`
- `step.started`
- `step.updated`
- `step.completed`
- `artifact.updated`
- `run.completed`
- `run.failed`

## Envelope

```json
{
  "type": "step.completed",
  "runId": "run_123",
  "timestamp": "2026-04-19T21:30:00Z",
  "payload": {
    "stepId": "step_456",
    "label": "检索语雀知识库",
    "summary": "Matched 4 chunks from 2 repositories."
  }
}
```

## Recovery Model

The mobile client should:

1. Subscribe to SSE for live updates.
2. Re-fetch `GET /v1/runs/:id` after reconnect to rebuild state from the source
   of truth.
3. Treat SSE as live hints, not the only persistence mechanism.

# FluxA Model Groups Diagnostics

## Goal

Determine why an authenticated FluxA user receives a model-groups load failure without exposing credentials or access tokens.

## Design

The FluxA upstream client will emit a structured diagnostic only when a request cannot produce a usable response. The diagnostic contains the endpoint path, HTTP status (when present), and a response-shape classification. It never contains request headers, response bodies, usernames, passwords, or tokens.

The existing public API contract remains unchanged: upstream authentication expiry maps to re-authentication required; other upstream failures map to service unavailable.

## Validation

Add unit coverage proving diagnostic classification and preserving the existing error mapping. Restart the local in-memory API, sign in again on the device, reload model groups, and use the safe diagnostic to implement the smallest compatible parser or endpoint correction.

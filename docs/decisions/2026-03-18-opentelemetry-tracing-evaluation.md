# ADR: OpenTelemetry Distributed Tracing Evaluation

**Status**: Deferred
**Date**: 2026-03-18
**Author**: Claude Code (spike for #413)

## Context

North Cloud has 14 Go services communicating via HTTP, Elasticsearch, Redis pub/sub, and PostgreSQL. When investigating production incidents, tracing a request across services (e.g., crawler -> classifier -> publisher -> Redis) requires manual log correlation using timestamps and content IDs.

## Evaluation Summary

### OpenTelemetry Go SDK

- **SDK**: `go.opentelemetry.io/otel` (stable v1.x)
- **Exporters**: OTLP (to Jaeger, Tempo, etc.), stdout, Zipkin
- **Auto-instrumentation**: Available for `net/http`, `database/sql`, `elasticsearch`, `gin-gonic/gin`
- **Propagation**: W3C TraceContext headers (`traceparent`, `tracestate`)

### Instrumentation Effort Per Service

| Component | Effort | Notes |
|-----------|--------|-------|
| SDK init + shutdown | ~30 lines in bootstrap | One-time per service |
| HTTP middleware (Gin) | ~5 lines | `otelgin.Middleware()` auto-instruments all routes |
| ES client wrapping | ~20 lines | Manual span creation around ES calls |
| Redis pub/sub | Medium | No auto-instrumentation; inject trace context into message headers |
| PostgreSQL | ~5 lines | `otelsql` wraps `database/sql` driver |
| Cross-service HTTP | ~5 lines | `otelhttp.NewTransport()` wraps `http.Client` |

**Estimated total**: ~2-3 days for full instrumentation across all 14 services.

### Trace Propagation Points

1. **HTTP (service-to-service)**: W3C TraceContext via `otelhttp` transport — automatic
2. **Elasticsearch**: Manual span creation; no built-in trace propagation in ES protocol
3. **Redis pub/sub**: Inject `traceparent` as a message field; consumer extracts and links spans
4. **PostgreSQL**: `otelsql` wraps driver transparently

### Key Challenge: Redis Pub/Sub Gap

The publisher -> consumer path uses Redis pub/sub which has no native trace propagation. To maintain trace continuity:
- Publisher must serialize trace context into the Redis message JSON
- Consumers (Streetcode Laravel, etc.) must extract and create linked spans
- This crosses language boundaries (Go -> PHP), adding complexity

### Backend Options

| Backend | Self-hosted | Cloud | Cost | Integration |
|---------|------------|-------|------|-------------|
| Grafana Tempo | Yes (we run Grafana) | Grafana Cloud | Free tier available | Native Grafana integration |
| Jaeger | Yes | No | Free | Standalone UI |
| Zipkin | Yes | No | Free | Lightweight |

**Tempo is the natural choice** — we already run Grafana + Loki. Tempo adds trace correlation to existing log queries via TraceID linking.

## Decision: Defer

### Rationale

1. **Current observability is sufficient**: Prometheus metrics (just added) + Loki structured logs with `request_id` fields cover 90% of debugging needs. Most incidents are single-service (classifier stuck, publisher lag) not cross-service trace mysteries.

2. **Cost vs. benefit**: 2-3 days of instrumentation + Tempo deployment + Redis trace propagation + PHP consumer changes is significant. The ROI is low given our current incident patterns.

3. **Redis pub/sub boundary**: The Go -> PHP trace gap means we'd get partial traces anyway. Full value requires changes to Streetcode Laravel and other consumers.

4. **Prometheus metrics are the higher-value investment**: Request latency histograms, error rates, and queue depths answer "is something wrong?" faster than traces answer "what went wrong?".

### What Would Trigger Revisiting

- **Cross-service latency issues**: If we see requests that are slow end-to-end but fast within each service, traces become essential
- **Service mesh adoption**: If we move to a service mesh (Istio, Linkerd), tracing comes nearly free
- **Team growth**: More developers means more need for self-service debugging tools
- **Consumer diversification**: If we add more Redis consumers beyond Streetcode, trace propagation becomes more valuable

### Recommended Pre-work (Low Cost)

Even while deferring full tracing, we can prepare:

1. **Add `request_id` to all inter-service HTTP calls** — already partially done via middleware
2. **Include `content_id` in all log entries** for the content pipeline path — enables manual correlation
3. **Use Loki's derived fields** to link log entries by `request_id` in Grafana — gets 80% of trace value

## References

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Grafana Tempo](https://grafana.com/docs/tempo/latest/)
- [otelgin middleware](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin)

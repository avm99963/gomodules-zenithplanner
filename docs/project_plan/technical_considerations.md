# Technical Considerations

## Core Technologies

* **Backend:** Go service exposing an **HTTPS endpoint** for webhooks, handling API interactions, database cache updates, **SMTP email sending**, and **scheduled background tasks** including reconciliation. **Does not serve a statistics UI.**
* **Database:** **PostgreSQL**. Stores schedule entries, event cache, and sync state.
* **Visualization:** Existing Grafana instance.
* **Deployment:** **Containerized** (e.g., using Docker) Go backend service and PostgreSQL database, requiring hosting. The base URL for the webhook endpoint must be publicly accessible via HTTPS.
* **Build system:** **Bazel 8.2.1** with Bzlmod.

## Authentication

* **Google Calendar:** OAuth 2.0 for secure access by the Go backend (requiring read/write permissions for Calendar API). Initial authorization handled via a **command-line flow**; the resulting refresh token provided via `GOOGLE_REFRESH_TOKEN`.
* **SMTP:** Credentials (host, port, user, password) provided via environment variables.

## Authoritative Data Storage

Internal PostgreSQL database. **The `schedule_entries` table reflects the desired state derived from the `calendar_event_cache`, which is updated based on Google Calendar changes.**

* **Schema Idea:**
    * Table schedule\_entries (for Grafana/stats): date (DATE, PK), location\_code (TEXT), status (TEXT), is\_default (BOOLEAN).
    * Table calendar\_event\_cache: event\_id (TEXT, PK), date (DATE, Index), title (TEXT), updated\_ts (TIMESTAMPZ, Index), is\_managed\_property (BOOLEAN), is\_managed\_description (BOOLEAN), color\_id (TEXT), recurring\_event\_id (TEXT, Nullable, Index), original\_start\_time (TIMESTAMPZ, Nullable).
    * Table sync\_state: key (TEXT, PK), value (TEXT).

### Configuration (via Environment Variables)

* **Google Integration:** GOOGLE\_CALENDAR\_ID, APP\_BASE\_URL, WEBHOOK\_VERIFICATION\_TOKEN, GOOGLE\_REFRESH\_TOKEN, *(Optional)* GOOGLE\_CLIENT\_ID, GOOGLE\_CLIENT\_SECRET.
* **Application Logic:** DEFAULT\_LOCATION\_CODE, FUTURE\_HORIZON\_DAYS, ENABLE\_EMAIL\_CONFIRMATIONS, PAST\_SYNC\_WINDOW\_DAYS (Number of past days to include in full sync reconciliation window, e.g., "30").
* **Database:** DB\_HOST, DB\_PORT, DB\_USER, DB\_PASSWORD, DB\_NAME, DB\_SSLMODE.
* **SMTP Email:** SMTP\_HOST, SMTP\_PORT, SMTP\_USER, SMTP\_PASSWORD, SMTP\_SENDER\_ADDRESS, RECIPIENT\_EMAIL\_ADDRESS.

### Responsabilities of the backend

* **Webhook Channel Management:** (As described previously - Startup check, Daily renewal)
* **Background Task (Horizon Maintenance):**
  * A scheduled job (e.g., running **daily**) is required.
  * **Function:** Calculates future date range. Queries `calendar_event_cache` for managed events in the range. For dates missing a managed event, triggers the Reconciliation process for that date (providing nil as the cached event) to ensure default DB entry and Calendar event creation.
  * **Idempotency:** Required.
* **Synchronization & Reconciliation Logic:**
  * **Incremental Sync (Webhooks):** Fetches changed events using `syncToken`. Updates the `calendar_event_cache`. Handles 410 GONE by triggering full sync. Triggers Reconciliation for affected dates. Persists new `syncToken`.
  * **Full Sync Mechanism:** Fetches **all** events (no `syncToken`). Clears and rebuilds the `calendar_event_cache`. Persists new `syncToken`. Triggers Reconciliation for a window covering today - `PAST_SYNC_WINDOW_DAYS` to today + `FUTURE_HORIZON_DAYS`. Runs on startup and weekly.
  * **Reconciliation Process (Triggered after Cache Update):**
    * Takes a set of dates to process as input.
    * For each date:
      * Queries `calendar_event_cache` to find managed events.
      * Applies conflict resolution (latest `updated_ts` from cache) to identify the single `authoritative_cached_event`.
      * **Cleanup:** Deletes other managed events/instances for that date from Google Calendar via API.
      * **Reconcile:** Compares `authoritative_cached_event` (or lack thereof) with the `schedule_entries` table. Updates `schedule_entries` and Calendar metadata. Creates default Calendar event if required.
    * Sends summary email if changes occurred and emails are enabled (typically only for Full Sync trigger).
  * **Webhook Feedback Loop:** Reconciliation actions will trigger new webhooks. Subsequent incremental sync/reconciliation should ideally be a NOOP.
  * Error handling, idempotency crucial.
* **Email Sending:** Via **SMTP** after reconciliation confirms user-initiated changes (processed via incremental sync) or significant sync corrections (processed via full sync), **only if enabled via configuration**.

## Proposed Code Repository Structure

A standard Go project layout is recommended for maintainability. **Bazel 8.2.1 with Bzlmod enabled** will be used for building and dependency management.

```
zenithplanner/
├── cmd/
│   ├── backend/
│   │   ├── main.go
│   │   └── BUILD.bazel
│   └── oauthcli/
│       ├── main.go
│       └── BUILD.bazel
├── internal/
│   ├── calendar/
│   │   ├── \*.go
│   │   └── BUILD.bazel
│   # .. other internal packages with their *.go files and BUILD.bazel...
│   ├── config/
│   ├── database/
│   ├── email/
│   ├── handler/
│   ├── scheduler/
│   └── sync/
├── configs/            # Example configuration files (e.g., .env.example)
├── scripts/            # Helper scripts (build, deploy, etc.)
├── Dockerfile          # Dockerfile for the Go backend service (likely multi-stage using Bazel)
├── docker-compose.yml  # Docker Compose for local dev (backend \+ postgres)
├── go.mod              # Go module definition (used by Bzlmod)
├── go.sum              # Go module checksums
├── MODULE.bazel        # Bzlmod main module definition
├── WORKSPACE           # Minimal WORKSPACE file (needed by Bazel, often empty with Bzlmod)
└── BUILD.bazel         # Root BUILD file (optional, can define project-wide settings)
```

* **cmd/**: Contains the main applications. Each has its own BUILD.bazel file.
* **internal/**: Holds the core logic packages. Each package has its own BUILD.bazel file defining targets (libraries, binaries).
* **configs/**: Example configuration files.
* **scripts/**: Utility scripts.
* **Dockerfile / docker-compose.yml**: Containerization setup.
* go.mod **/** go.sum: Standard Go module files, used by Bazel's Go rules via Bzlmod.
* MODULE.bazel: Defines the Bazel module, dependencies (Go SDK, external Go libraries via `go_deps.from_file(go_mod = '//:go.mod')`, other Bazel modules).
* WORKSPACE: Minimal file required by Bazel; dependency management moves primarily to MODULE.bazel.
* BUILD.bazel **files**: Define build targets (`go_library`, `go_binary`, `go_test`) within each package.

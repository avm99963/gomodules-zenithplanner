# Proposed Technical Architecture Summary

ZenithPlanner operates as a backend synchronization service for Google Calendar schedules. Users primarily manage their schedules directly within **Google Calendar**.

## Backend System

* **Technology:** A **Go backend service**, running as a containerized application.
* **Event Detection:** Listens for changes on the user's designated Google Calendar via **HTTPS webhooks**.
* **Database:** Uses a **PostgreSQL database** for internal storage.

### Data Management & Synchronization

* **Internal Cache:** Maintains a `calendar_event_cache` within PostgreSQL.
    * This cache reflects the state of relevant Google Calendar events, including expanded instances of recurring events.
* **Sync Operations:** The cache is updated via:
    * Incremental syncs triggered by webhooks.
    * Incremental (or full) syncs performed on service startup or on a defined schedule.

### Reconciliation Process

* **Input:** Reads data from the `calendar_event_cache`.
* **Actions:**
    * Performs cleanup tasks on Google Calendar (e.g., deleting duplicate events managed by ZenithPlanner).
    * Updates the main `schedule_entries` table, which is used for generating statistics.
    * Updates Google Calendar event metadata (like colors and custom tags) to match the state derived from the cache.

### Additional Backend Tasks

* Proactively creates default daily events within Google Calendar.
* Manages the lifecycle of webhook subscriptions.
* Can send optional confirmation emails using **SMTP**.

## Statistics & Visualization

* Statistics are derived from the `schedule_entries` table in the PostgreSQL database.
* An existing **Grafana** instance connects to this table to visualize the statistics.

## Build System

* The project uses **Bazel** as its build system.
* It utilizes **Bazel Modules (Bzlmod)** for dependency management.

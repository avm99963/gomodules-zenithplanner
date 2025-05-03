# Glossary

* **Managed Event:** A Google Calendar event (typically all-day) on the configured calendar that ZenithPlanner actively tracks and synchronizes. Identified by a private extended property or a specific description tag.
* **Calendar Event Cache:** A table (calendar\_event\_cache) within the PostgreSQL database storing key details (ID, date, title, updated timestamp, managed status, etc.) of potentially relevant events fetched from Google Calendar. Includes individual instances of recurring events.
* **Schedule Entries:** The main table (schedule\_entries) in the PostgreSQL database storing the final, reconciled state (location code, status) for each day. This table is the data source for Grafana statistics.
* **Synchronization (Sync):** The overall process of fetching changes from Google Calendar and updating the internal database cache (calendar\_event\_cache). Can be Incremental or Full.
* **Reconciliation:** The process triggered after a sync updates the cache. It reads the cache, cleans up duplicate managed events on Google Calendar, updates the schedule\_entries table, and ensures Calendar event metadata (color, properties) is correct based on the authoritative event determined from the cache.
* **Authoritative Event:** For any given day, the single *managed* event determined to be the correct one, based on conflict resolution rules (usually the latest updated timestamp).
* **Private Extended Property:** A hidden key-value pair (zenithplanner\_managed: "true") added to Google Calendar events by the backend to reliably identify them as managed by ZenithPlanner.
* **Description Tag:** An alternative way to initially mark a user-created event as managed (Add-To-ZenithPlanner: true in the description). The tag is removed, and the private property is added during the first reconciliation.
* **Horizon Maintenance:** A background task that ensures default managed events exist for a configurable period into the future.
* **Webhook:** An HTTPS endpoint exposed by the backend service that Google Calendar sends notifications to when changes occur.
* **Sync Token:** A token provided by the Google Calendar API used to fetch only the changes that occurred since the token was issued (used for incremental sync).

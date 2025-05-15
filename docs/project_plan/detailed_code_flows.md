# Detailed Code Flows (original plan)

*** note
**Warning:** the implementation currently deviates a lot from this original
plan. We only leave it here for historical reasons. Please don't use it to
understand how the project works, since it does NOT work in exactly this way.

You can make better use of your time by reading the code directly.
***

This section outlines the key operational flows of the ZenithPlanner backend service.

## Initial Authentication (OAuth CLI Tool)

**Goal:** Obtain the initial OAuth 2.0 refresh token required for the backend service to access Google Calendar API non-interactively.

**Trigger:** Manual execution of the oauthcli command-line tool.

**Steps:**

1. **Load Credentials:** CLI loads Google Client ID and Client Secret.
2. **Generate Auth URL:** Generate URL requesting Calendar read/write scope.
3. **User Interaction (Manual):** User visits URL, grants permission.
4. **Receive Code:** User copies authorization code.
5. **Exchange Code:** User pastes code into CLI.
6. **Token Exchange:** CLI exchanges code for tokens.
7. **Receive Tokens:** Google responds with access and refresh tokens.
8. **Store Refresh Token:** CLI outputs refresh token for `GOOGLE_REFRESH_TOKEN` env var.

## Incremental Sync (Webhook Triggered)

**Goal:** Update the internal event cache based on real-time Calendar changes and trigger reconciliation for affected dates.

**Trigger:** HTTPS POST from Google Calendar Push Notification service.

**Steps:**

1. **Receive Notification:** Webhook handler validates request.
2. **Fetch Changes:** Retrieve persisted syncToken. Use `events.list` API with `syncToken`. Handle 410 GONE by triggering full sync and stopping this flow.
3. **Update Cache:** For each changed event from API:
   * If it's a **recurring event master** that was updated/deleted: Identify all potential instance dates affected within a relevant window (past/future) and mark them as needing reconciliation. *(Cache update logic needs careful design)*.
   * If it's a **single instance** or **non-recurring event** (created, updated, deleted): UPSERT or DELETE the specific event in `calendar_event_cache`. Identify the affected date(s).
4. **Trigger Reconciliation:** For the set of unique affected dates, trigger the Reconciliation Process for those specific dates.
5. **Update Sync Token:** Persist new `syncToken` to DB.
6. **Acknowledge Request:** Respond HTTP 200 OK.

## Full Sync (Startup / Weekly Background Task / Triggered)

**Goal:** Completely refresh the internal event cache and trigger reconciliation for the full relevant time window.

**Trigger:** Application startup, weekly scheduled task, or 410 GONE.

**Steps:**

1. **Fetch Calendar Events:** Get **all events** via API **without `syncToken`** (including `singleEvents=true`). Paginate as needed.
2. **Rebuild Cache:**
   * DELETE **all** entries from `calendar_event_cache`.
   * Iterate through fetched events: Expand recurrences within a large practical window. For each relevant instance/event, INSERT its details into `calendar_event_cache`.
3. **Update Sync Token:** Persist the fresh `syncToken` obtained from the API response (from the last page of the fetch).
4. **Trigger Reconciliation:** Trigger the Reconciliation Process for a window covering today - `PAST_SYNC_WINDOW_DAYS` to today + `FUTURE_HORIZON_DAYS`.

## Reconciliation Process (Scheduled or Triggered)

**Goal:** Clean up Calendar based on cache, then update main DB (`schedule_entries`) and Calendar metadata.

**Trigger:** Called after Incremental or Full Sync Cache updates, or by Horizon Maintenance.

**Input:** datesToReconcile (Set of dates to process).

**Steps:**

1. **Initialize:** Create map `changesMade` to track dates for email summary. `userTriggeredChange = true` if called from Incremental Sync, `false` otherwise.
2. **Iterate Dates:** For each `date` in `datesToReconcile`:
   * **Query Cache:** Get all cached event entries (instances and single events) for `date` from `calendar_event_cache`.
   * **Identify Authoritative & Duplicates:** Filter for managed events. Apply conflict resolution (latest `updated_ts` from cache) to find the single `authoritative_cached_event` (can be `nil`) and a list `duplicates_to_delete` (containing event IDs of older managed events/instances for this `date`).
   * **Cleanup Calendar:** For each `eventId` in `duplicates_to_delete`, call `events.delete` API. Log deletion.
   * **Reconcile Core State:** Fetch `currentDbEntry` from `schedule_entries` for `date`. Call Core Reconciliation Logic with `date`, `authoritative_cached_event` data, `currentDbEntry`.
   * **Track Changes:** If reconciliation updated the `schedule_entries` DB, add date and change details to `changesMade`.
3. **Send Email:** If `changesMade` is not empty and emails enabled:
   * If `userTriggeredChange` is `true` (from Incremental Sync), send the appropriate single/recurring change email.
   * If `userTriggeredChange` is `false` (from Full Sync/Horizon), send the "Full Sync Changes" summary email.

## Horizon Maintenance (Daily Background Task)

**Goal:** Ensure default, tagged events exist for the future horizon by triggering reconciliation for missing days.

**Trigger:** Daily scheduled task.

**Steps:**

1. **Define Window:**`today` to `today + FUTURE_HORIZON_DAYS`.
2. **Query Cache:** Query `calendar_event_cache` for all dates within the window that have at least one managed event entry.
3. **Identify Missing Dates:** Determine the set of dates within the window that are *not* present in the cache query results.
4. **Trigger Reconciliation for Missing Dates:** Trigger the Reconciliation Process with the `missingDates` set as input. This will invoke the Core Reconciliation Logic with `authoritativeCachedEventData = nil`, leading to default event creation if needed.

## Webhook Channel Renewal (Scheduled Task)

**Goal:** Prevent the Google Calendar push notification channel from expiring.

**Trigger:** Daily scheduled task.

**Steps:**

1. **Load Channel Info:** Get active channel ID, resource ID, expiration from DB.
2. **Check Expiration:** Nearing expiration?
3. **Renew if Needed:**
   * Generate a **new unique channel ID** (e.g., UUID).
   * Construct the full webhook URL.
   * Call events.watch with the **new ID**, the `WEBHOOK_VERIFICATION_TOKEN`, and the webhook URL.
   * Store the **new** channel ID, resource ID, and expiration time in the DB, replacing the old ones.
   * (Optional) Call `channels.stop` using the *old* channel ID and resource ID to clean up.
4. **Log Result.**

## Core Reconciliation Logic (for a Single Day)

**Goal:** Ensure the `schedule_entries` table and the single authoritative Calendar event's metadata are consistent. Assumes Calendar cleanup already happened for this date.

**Trigger:** Called by the Reconciliation Process.

**Inputs:** `date`, `authoritativeCachedEventData` (data from the cache for the single latest managed event/instance, could be `nil`), `currentDbEntry` (from `schedule_entries`, could be `nil`).

**Steps:**

1. **Determine Target State & Required Actions:**
   * **Case 1: Authoritative Cached Event Exists (`authoritativeCachedEventData != nil`)**
     * Parse `title` -> `targetLocationCode`. Determine `targetStatus`.
     * `needsProperty = true` if cache indicates event lacks private property.
     * `needsDescriptionUpdate = true` if cache indicates event contains description tag.
     * `needsColorUpdate = true` if cached color \!= expected color.
     * `needsDbUpdate = true` if `currentDbEntry` is `nil`, differs from target, or `is_default` is true.
     * `isTargetDefault = false`.
     * `eventId = authoritativeCachedEventData.event_id`.
   * **Case 2: No Authoritative Cached Event (`authoritativeCachedEventData == nil`)**
     * `targetLocationCode = DEFAULT_LOCATION_CODE`. `targetStatus` = Default status.
     * `needsEventCreation = true` only if `currentDbEntry` is `nil` or not default.
     * `needsDbUpdate = true` if `currentDbEntry` is `nil` or not default.
     * `isTargetDefault = true`.
     * `eventId = nil`.
2. **Database Update (`schedule_entries`):**
   * If `needsDbUpdate`: UPSERT `schedule_entries` with target state. Mark DB as changed for this date.
3. **Calendar Metadata Updates (If Authoritative Event Exists):**
   * If `needsProperty`: Call Calendar API `events.patch` for `eventId` to add private property.
   * If `needsDescriptionUpdate`: Call Calendar API `events.patch` for `eventId` to update description (remove tag).
   * If `needsColorUpdate`: Call Calendar API `events.patch` for `eventId` to set correct color ID.
4. **Calendar Creation (If No Authoritative Event and Creation Needed):**
   * If `needsEventCreation`: Call Calendar API `events.insert` to create new default event (with title, color, private property). *(Cache Update Note: This newly created event will be picked up and added to the cache during the next sync, either incremental or full)*.
5. **Return Status:** Indicate whether `schedule_entries` DB was changed.

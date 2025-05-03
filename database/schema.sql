-- Table to store the final reconciled state for statistics (Grafana source)
CREATE TABLE IF NOT EXISTS schedule_entries (
    date DATE PRIMARY KEY,                  -- The specific date
    location_code TEXT NOT NULL,            -- The determined location code (e.g., 'HOM', 'V', 'P12...')
    status TEXT NOT NULL                    -- Status derived from code (e.g., 'Default', 'Vacation', 'Office', 'Library')
);

-- Table to cache relevant details fetched from Google Calendar events
-- This acts as an intermediate store before reconciliation.
CREATE TABLE IF NOT EXISTS calendar_event_cache (
    event_id TEXT PRIMARY KEY,                  -- Google Calendar event ID (unique ID for instances)
    date DATE NOT NULL,                         -- The specific date this instance/event applies to
    title TEXT,                                 -- Event title (summary)
    description TEXT,                           -- Event description (needed for tag removal)
    updated_ts TIMESTAMPTZ NOT NULL,            -- Last updated timestamp from Google Calendar
    is_managed_property BOOLEAN NOT NULL DEFAULT false, -- True if the event has the private property
    is_managed_description BOOLEAN NOT NULL DEFAULT false, -- True if the event has the description tag
    color_id TEXT,                              -- Google Calendar color ID
    recurring_event_id TEXT,                    -- ID of the master recurring event (if applicable)
    original_start_time TIMESTAMPTZ             -- Original start time (for recurring instances)
);

-- Add indexes for faster lookups on the cache table
CREATE INDEX IF NOT EXISTS idx_calendar_event_cache_date ON calendar_event_cache (date);
CREATE INDEX IF NOT EXISTS idx_calendar_event_cache_updated_ts ON calendar_event_cache (updated_ts);
CREATE INDEX IF NOT EXISTS idx_calendar_event_cache_recurring_id ON calendar_event_cache (recurring_event_id);


-- Table to store synchronization state (e.g., sync token, webhook channel info)
CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,   -- e.g., 'syncToken', 'channelId', 'resourceId', 'channelExpiration'
    value TEXT NOT NULL     -- The corresponding value
);



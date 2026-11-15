package conversation

const nativeSchema = `
CREATE TABLE native_source (
 id TEXT PRIMARY KEY, path TEXT NOT NULL, generation INTEGER NOT NULL,
 conversation_id TEXT NOT NULL REFERENCES conversation(id) ON DELETE CASCADE,
 provider_thread_id TEXT NOT NULL REFERENCES provider_thread(id) ON DELETE CASCADE,
 native_id TEXT NOT NULL, parser TEXT NOT NULL, consumed INTEGER NOT NULL,
 checksum TEXT NOT NULL, file_identity TEXT NOT NULL,
 native_time TEXT NOT NULL, model TEXT NOT NULL, model_provider TEXT NOT NULL, cli_version TEXT NOT NULL, pending INTEGER NOT NULL, UNIQUE(path,generation)
);
CREATE INDEX native_source_identity ON native_source(file_identity,native_id,parser);
CREATE TABLE native_location (
 path TEXT PRIMARY KEY, source_id TEXT NOT NULL REFERENCES native_source(id) ON DELETE CASCADE
);
CREATE TABLE native_record (
 source_id TEXT NOT NULL REFERENCES native_source(id) ON DELETE CASCADE,
 offset INTEGER NOT NULL, end_offset INTEGER NOT NULL, checksum TEXT NOT NULL,
 activity_id TEXT REFERENCES activity(id) ON DELETE CASCADE,
 call_id TEXT NOT NULL, tool_name TEXT NOT NULL, record_kind TEXT NOT NULL,
 PRIMARY KEY(source_id,offset,activity_id)
);
CREATE TABLE history_snapshot (
 id TEXT PRIMARY KEY, source_id TEXT NOT NULL REFERENCES native_source(id) ON DELETE CASCADE,
 parser TEXT NOT NULL, consumed INTEGER NOT NULL, checksum TEXT NOT NULL,
 diagnostics BLOB NOT NULL, model TEXT NOT NULL
);
CREATE TABLE history_snapshot_item (
 snapshot_id TEXT NOT NULL REFERENCES history_snapshot(id) ON DELETE CASCADE,
 activity_id TEXT NOT NULL REFERENCES activity(id) ON DELETE CASCADE,
 position INTEGER NOT NULL, PRIMARY KEY(snapshot_id,position)
);
CREATE TABLE history_snapshot_reference (
 snapshot_id TEXT NOT NULL REFERENCES history_snapshot(id) ON DELETE CASCADE,
 owner TEXT NOT NULL, PRIMARY KEY(snapshot_id,owner)
);
PRAGMA user_version=12;
`

package conversation

// The v13 migration freezes existing committed bodies before this clean cutover.
// Only explicit immutable exports remain; live native files are never mirrored.
const nativeCutoverSchema = `
ALTER TABLE frozen_export ADD COLUMN text_bytes INTEGER NOT NULL DEFAULT 0;
UPDATE frozen_export SET text_bytes=(SELECT COALESCE(sum(length(CAST(json_extract(r.value,'$.Text') AS BLOB))),0)
 FROM json_each(frozen_export.body,'$.Records') r);
DROP TABLE history_snapshot_reference;
DROP TABLE history_snapshot_gap;
DROP TABLE history_snapshot_item;
DROP TABLE history_snapshot;
DROP TABLE native_record;
DROP TABLE native_location;
DROP TABLE native_source;
DROP TABLE activity_attachment;
DROP TABLE activity_heading_chunk;
DROP TABLE activity_chunk;
DROP TABLE activity;
CREATE TRIGGER frozen_handoff_release AFTER DELETE ON context_handoff BEGIN
 DELETE FROM frozen_reference WHERE owner=OLD.id;
END;
CREATE TRIGGER frozen_conversation_release AFTER DELETE ON conversation BEGIN
 DELETE FROM frozen_reference WHERE owner=OLD.id;
END;
PRAGMA user_version=14;
`

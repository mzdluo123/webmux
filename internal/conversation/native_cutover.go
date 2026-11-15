package conversation

// The v13 migration freezes existing committed bodies before this clean cutover.
// Only explicit immutable exports remain; live native files are never mirrored.
const nativeCutoverSchema = `
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

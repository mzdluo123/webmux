-- Real native schema and sanitized excerpts; import into a new empty temporary database.
PRAGMA foreign_keys = ON;
BEGIN TRANSACTION;
CREATE TABLE `project` (
          `id` text PRIMARY KEY,
          `worktree` text NOT NULL,
          `vcs` text,
          `name` text,
          `icon_url` text,
          `icon_url_override` text,
          `icon_color` text,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `time_initialized` integer,
          `sandboxes` text NOT NULL,
          `commands` text
        , `time_active` integer DEFAULT 0 NOT NULL, `time_archived` integer);
CREATE TABLE `session` (
          `id` text PRIMARY KEY,
          `project_id` text NOT NULL,
          `workspace_id` text,
          `parent_id` text,
          `slug` text NOT NULL,
          `directory` text NOT NULL,
          `path` text,
          `title` text NOT NULL,
          `version` text NOT NULL,
          `share_url` text,
          `summary_additions` integer,
          `summary_deletions` integer,
          `summary_files` integer,
          `summary_diffs` text,
          `metadata` text,
          `cost` real DEFAULT 0 NOT NULL,
          `tokens_input` integer DEFAULT 0 NOT NULL,
          `tokens_output` integer DEFAULT 0 NOT NULL,
          `tokens_reasoning` integer DEFAULT 0 NOT NULL,
          `tokens_cache_read` integer DEFAULT 0 NOT NULL,
          `tokens_cache_write` integer DEFAULT 0 NOT NULL,
          `revert` text,
          `permission` text,
          `agent` text,
          `model` text,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `time_compacting` integer,
          `time_archived` integer,
          CONSTRAINT `fk_session_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE INDEX `session_parent_idx` ON `session` (`parent_id`);
CREATE INDEX `session_project_idx` ON `session` (`project_id`);
CREATE INDEX `session_workspace_idx` ON `session` (`workspace_id`);
CREATE TABLE `message` (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_message_session_id_session_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session`(`id`) ON DELETE CASCADE
        );
CREATE INDEX `message_session_time_created_id_idx` ON `message` (`session_id`,`time_created`,`id`);
CREATE TABLE `part` (
          `id` text PRIMARY KEY,
          `message_id` text NOT NULL,
          `session_id` text NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_part_message_id_message_id_fk` FOREIGN KEY (`message_id`) REFERENCES `message`(`id`) ON DELETE CASCADE
        );
CREATE INDEX `part_message_id_id_idx` ON `part` (`message_id`,`id`);
CREATE INDEX `part_session_idx` ON `part` (`session_id`);
INSERT INTO `project` (`id`,`worktree`,`vcs`,`name`,`icon_url`,`icon_url_override`,`icon_color`,`time_created`,`time_updated`,`time_initialized`,`sandboxes`,`commands`,`time_active`,`time_archived`) VALUES ('0000000000000000000000000000000000000001','/fixture/workspace',NULL,NULL,NULL,NULL,NULL,1700000000000,1700000000069,NULL,'[]',NULL,1700000000069,NULL);
INSERT INTO `session` (`id`,`project_id`,`workspace_id`,`parent_id`,`slug`,`directory`,`path`,`title`,`version`,`share_url`,`summary_additions`,`summary_deletions`,`summary_files`,`summary_diffs`,`metadata`,`cost`,`tokens_input`,`tokens_output`,`tokens_reasoning`,`tokens_cache_read`,`tokens_cache_write`,`revert`,`permission`,`agent`,`model`,`time_created`,`time_updated`,`time_compacting`,`time_archived`) VALUES ('ses_fixture_legacy','0000000000000000000000000000000000000001',NULL,NULL,'[redacted]','/fixture/path-2','/fixture/path-3','[redacted]','1.18.3',NULL,0,0,0,NULL,NULL,0.0,36893,7296,0,278528,0,NULL,NULL,'[redacted]','{"id":"[redacted]","providerID":"[redacted]"}',1700000000001,1700000000032,NULL,NULL);
INSERT INTO `message` (`id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_legacy_0001','ses_fixture_legacy',1700000000002,1700000000031,'{"role":"user","time":{"created":1700000000002},"agent":"[redacted]","model":{"providerID":"[redacted]","modelID":"[redacted]"},"summary":{"diffs":[]}}');
INSERT INTO `message` (`id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_legacy_0002','ses_fixture_legacy',1700000000005,1700000000017,'{"parentID":"msg_fixture_legacy_0001","role":"assistant","mode":"[redacted]","agent":"[redacted]","path":{"cwd":"/fixture/path-2","root":"/fixture/workspace"},"cost":0,"tokens":{"total":7891,"input":7816,"output":75,"reasoning":0,"cache":{"write":0,"read":0}},"modelID":"[redacted]","providerID":"[redacted]","time":{"created":1700000000005,"completed":1700000000017},"finish":"tool-calls"}');
INSERT INTO `message` (`id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_legacy_0003','ses_fixture_legacy',1700000000018,1700000000030,'{"parentID":"msg_fixture_legacy_0001","role":"assistant","mode":"[redacted]","agent":"[redacted]","path":{"cwd":"/fixture/path-2","root":"/fixture/workspace"},"cost":0,"tokens":{"total":8023,"input":744,"output":111,"reasoning":0,"cache":{"write":0,"read":7168}},"modelID":"[redacted]","providerID":"[redacted]","time":{"created":1700000000018,"completed":1700000000029},"finish":"stop"}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0001','msg_fixture_legacy_0001','ses_fixture_legacy',1700000000003,1700000000004,'{"type":"text","text":"[redacted]"}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0002','msg_fixture_legacy_0002','ses_fixture_legacy',1700000000006,1700000000007,'{"type":"step-start"}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0003','msg_fixture_legacy_0002','ses_fixture_legacy',1700000000009,1700000000011,'{"type":"reasoning","text":"[redacted]","time":{"start":1700000000008,"end":1700000000010}}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0004','msg_fixture_legacy_0002','ses_fixture_legacy',1700000000012,1700000000015,'{"type":"tool","tool":"bash","callID":"call_fixture_0001","state":{"status":"completed","input":{"command":"[redacted]"},"output":"[redacted]","metadata":{"output":"[redacted]","exit":128,"truncated":false},"title":"[redacted]","time":{"start":1700000000013,"end":1700000000014}}}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0005','msg_fixture_legacy_0002','ses_fixture_legacy',1700000000016,1700000000016,'{"reason":"tool-calls","type":"step-finish","tokens":{"total":7891,"input":7816,"output":75,"reasoning":0,"cache":{"write":0,"read":0}},"cost":0}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0006','msg_fixture_legacy_0003','ses_fixture_legacy',1700000000019,1700000000020,'{"type":"step-start"}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0007','msg_fixture_legacy_0003','ses_fixture_legacy',1700000000021,1700000000023,'{"type":"reasoning","text":"[redacted]","time":{"start":1700000000021,"end":1700000000022}}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0008','msg_fixture_legacy_0003','ses_fixture_legacy',1700000000024,1700000000026,'{"type":"text","text":"[redacted]","time":{"start":1700000000024,"end":1700000000025}}');
INSERT INTO `part` (`id`,`message_id`,`session_id`,`time_created`,`time_updated`,`data`) VALUES ('prt_fixture_legacy_0009','msg_fixture_legacy_0003','ses_fixture_legacy',1700000000027,1700000000028,'{"reason":"stop","type":"step-finish","tokens":{"total":8023,"input":744,"output":111,"reasoning":0,"cache":{"write":0,"read":7168}},"cost":0}');
COMMIT;

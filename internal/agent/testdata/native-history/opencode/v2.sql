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
CREATE TABLE `session_v2` (
          `id` text PRIMARY KEY,
          `project_id` text NOT NULL,
          `workspace_id` text,
          `parent_id` text,
          `fork_session_id` text,
          `fork_boundary` text,
          `slug` text NOT NULL,
          `directory` text NOT NULL,
          `path` text,
          `title` text,
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
          `time_suspended` integer, `resume_attempts` integer DEFAULT 0 NOT NULL, `time_idle` integer, `time_viewed` integer, `idle_outcome` text,
          CONSTRAINT `fk_session_v2_project_id_project_id_fk` FOREIGN KEY (`project_id`) REFERENCES `project`(`id`) ON DELETE CASCADE
        );
CREATE INDEX `session_v2_parent_idx` ON `session_v2` (`parent_id`);
CREATE INDEX `session_v2_project_idx` ON `session_v2` (`project_id`);
CREATE INDEX `session_v2_time_suspended_idx` ON `session_v2` (`time_suspended`) WHERE "session_v2"."time_suspended" is not null;
CREATE INDEX `session_v2_workspace_idx` ON `session_v2` (`workspace_id`);
CREATE TABLE "session_message" (
          `id` text PRIMARY KEY,
          `session_id` text NOT NULL,
          `type` text NOT NULL,
          `seq` integer NOT NULL,
          `time_created` integer NOT NULL,
          `time_updated` integer NOT NULL,
          `data` text NOT NULL,
          CONSTRAINT `fk_session_message_session_id_session_v2_id_fk` FOREIGN KEY (`session_id`) REFERENCES `session_v2`(`id`) ON DELETE CASCADE
        );
CREATE UNIQUE INDEX `session_message_session_seq_idx` ON `session_message` (`session_id`,`seq`);
CREATE INDEX `session_message_session_time_created_id_idx` ON `session_message` (`session_id`,`time_created`,`id`);
CREATE INDEX `session_message_session_type_seq_idx` ON `session_message` (`session_id`,`type`,`seq`);
CREATE INDEX `session_message_time_created_idx` ON `session_message` (`time_created`);
INSERT INTO `project` (`id`,`worktree`,`vcs`,`name`,`icon_url`,`icon_url_override`,`icon_color`,`time_created`,`time_updated`,`time_initialized`,`sandboxes`,`commands`,`time_active`,`time_archived`) VALUES ('0000000000000000000000000000000000000001','/fixture/workspace','git',NULL,NULL,NULL,NULL,1700000000070,1700000000070,NULL,'[]',NULL,1700000000088,NULL);
INSERT INTO `session_v2` (`id`,`project_id`,`workspace_id`,`parent_id`,`fork_session_id`,`fork_boundary`,`slug`,`directory`,`path`,`title`,`version`,`share_url`,`summary_additions`,`summary_deletions`,`summary_files`,`summary_diffs`,`metadata`,`cost`,`tokens_input`,`tokens_output`,`tokens_reasoning`,`tokens_cache_read`,`tokens_cache_write`,`revert`,`permission`,`agent`,`model`,`time_created`,`time_updated`,`time_compacting`,`time_archived`,`time_suspended`,`resume_attempts`,`time_idle`,`time_viewed`,`idle_outcome`) VALUES ('ses_fixture_v2','0000000000000000000000000000000000000001',NULL,NULL,NULL,NULL,'[redacted]','/fixture/path-2','/fixture/path-3','[redacted]','2.1.4',NULL,NULL,NULL,NULL,NULL,NULL,0.030840691999999996,93245,8368,6426,2667264,0,NULL,NULL,'[redacted]','{"id":"[redacted]","providerID":"[redacted]","variant":"[redacted]"}',1700000000071,1700000000085,NULL,NULL,NULL,0,1700000000087,NULL,'[redacted]');
INSERT INTO `session_message` (`id`,`session_id`,`type`,`seq`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_v2_0001','ses_fixture_v2','user',507,1700000000072,1700000000073,'{"time":{"created":1700000000072},"text":"[redacted]","files":[]}');
INSERT INTO `session_message` (`id`,`session_id`,`type`,`seq`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_v2_0002','ses_fixture_v2','assistant',508,1700000000074,1700000000082,'{"time":{"created":1700000000074,"streamed":1700000000079,"completed":1700000000081},"agent":"[redacted]","model":{"id":"[redacted]","providerID":"[redacted]","variant":"[redacted]"},"content":[{"type":"reasoning","text":"[redacted]","native":{"reasoningField":"reasoning_content"},"time":{"created":1700000000075,"completed":1700000000077}},{"type":"tool","id":"id_fixture_0001","name":"[redacted]","executed":false,"state":{"status":"completed","input":{"command":"[redacted]","timeout":120000},"content":[{"type":"text","text":"[redacted]"}],"metadata":{"status":"completed","truncated":false,"exit":0}},"time":{"created":1700000000076,"ran":1700000000078,"completed":1700000000080}}],"snapshot":{"start":"0000000000000000000000000000000000000001","end":"0000000000000000000000000000000000000001","files":[]},"finish":"tool-calls","rawFinish":"tool_calls","cost":0.000463908,"tokens":{"input":306,"output":162,"reasoning":79,"cache":{"read":91136,"write":0}}}');
INSERT INTO `session_message` (`id`,`session_id`,`type`,`seq`,`time_created`,`time_updated`,`data`) VALUES ('msg_fixture_v2_0003','ses_fixture_v2','assistant',517,1700000000083,1700000000086,'{"time":{"created":1700000000083,"streamed":1700000000084,"completed":1700000000085},"agent":"[redacted]","model":{"id":"[redacted]","providerID":"[redacted]","variant":"[redacted]"},"content":[{"type":"text","text":"[redacted]"}],"snapshot":{"start":"0000000000000000000000000000000000000001","end":"0000000000000000000000000000000000000001","files":[]},"finish":"stop","rawFinish":"stop","cost":0.00037152599999999995,"tokens":{"input":341,"output":77,"reasoning":0,"cache":{"read":91392,"write":0}}}');
COMMIT;

-- +goose Up
-- Preserve existing ID strings while accepting new shortuuid IDs.
ALTER TABLE api_keys DROP CONSTRAINT api_keys_user_id_fkey;
ALTER TABLE attributes DROP CONSTRAINT attributes_object_id_fkey;
ALTER TABLE attributes DROP CONSTRAINT attributes_target_object_id_fkey;
ALTER TABLE connections DROP CONSTRAINT connections_user_id_fkey;
ALTER TABLE connections DROP CONSTRAINT connections_workspace_id_fkey;
ALTER TABLE domain_rules DROP CONSTRAINT domain_rules_workspace_id_fkey;
ALTER TABLE gmail_drafts DROP CONSTRAINT gmail_drafts_connection_id_fkey;
ALTER TABLE gmail_drafts DROP CONSTRAINT gmail_drafts_follow_up_id_fkey;
ALTER TABLE handles DROP CONSTRAINT handles_person_id_fkey;
ALTER TABLE handles DROP CONSTRAINT handles_workspace_id_fkey;
ALTER TABLE identities DROP CONSTRAINT identities_user_id_fkey;
ALTER TABLE interactions DROP CONSTRAINT interactions_connection_id_fkey;
ALTER TABLE interactions DROP CONSTRAINT interactions_user_id_fkey;
ALTER TABLE interactions DROP CONSTRAINT interactions_workspace_id_fkey;
ALTER TABLE links DROP CONSTRAINT links_interaction_id_fkey;
ALTER TABLE links DROP CONSTRAINT links_record_id_fkey;
ALTER TABLE oauth_codes DROP CONSTRAINT oauth_codes_user_id_fkey;
ALTER TABLE oauth_grants DROP CONSTRAINT oauth_grants_user_id_fkey;
ALTER TABLE oauth_tokens DROP CONSTRAINT oauth_tokens_grant_id_fkey;
ALTER TABLE objects DROP CONSTRAINT objects_workspace_id_fkey;
ALTER TABLE participants DROP CONSTRAINT participants_handle_id_fkey;
ALTER TABLE participants DROP CONSTRAINT participants_interaction_id_fkey;
ALTER TABLE parts DROP CONSTRAINT parts_author_handle_id_fkey;
ALTER TABLE parts DROP CONSTRAINT parts_connection_id_fkey;
ALTER TABLE parts DROP CONSTRAINT parts_interaction_id_fkey;
ALTER TABLE record_values DROP CONSTRAINT record_values_actor_id_fkey;
ALTER TABLE record_values DROP CONSTRAINT record_values_attribute_id_fkey;
ALTER TABLE record_values DROP CONSTRAINT record_values_record_id_fkey;
ALTER TABLE record_values DROP CONSTRAINT record_values_ref_record_id_fkey;
ALTER TABLE records DROP CONSTRAINT records_object_id_fkey;
ALTER TABLE records DROP CONSTRAINT records_workspace_id_fkey;
ALTER TABLE saved_filters DROP CONSTRAINT saved_filters_object_id_fkey;
ALTER TABLE saved_filters DROP CONSTRAINT saved_filters_workspace_id_fkey;
ALTER TABLE sessions DROP CONSTRAINT sessions_user_id_fkey;
ALTER TABLE sync_cursors DROP CONSTRAINT sync_cursors_connection_id_fkey;
ALTER TABLE users DROP CONSTRAINT users_workspace_id_fkey;
ALTER TABLE workspace_invites DROP CONSTRAINT workspace_invites_invited_by_fkey;
ALTER TABLE workspace_invites DROP CONSTRAINT workspace_invites_workspace_id_fkey;
ALTER TABLE workspace_knowledge_pages DROP CONSTRAINT workspace_knowledge_pages_page_id_fkey;
ALTER TABLE workspace_knowledge_pages DROP CONSTRAINT workspace_knowledge_pages_workspace_id_fkey;
ALTER TABLE workspaces DROP CONSTRAINT workspaces_company_page_id_fkey;
ALTER TABLE api_keys
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE attributes
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN object_id TYPE text USING object_id::text,
  ALTER COLUMN target_object_id TYPE text USING target_object_id::text;
ALTER TABLE connections
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE domain_rules
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text;
ALTER TABLE gmail_drafts
  ALTER COLUMN connection_id TYPE text USING connection_id::text,
  ALTER COLUMN follow_up_id TYPE text USING follow_up_id::text;
ALTER TABLE handles
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN person_id TYPE text USING person_id::text;
ALTER TABLE identities
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE interactions
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN connection_id TYPE text USING connection_id::text,
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE links
  ALTER COLUMN interaction_id TYPE text USING interaction_id::text,
  ALTER COLUMN record_id TYPE text USING record_id::text;
ALTER TABLE oauth_codes
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE oauth_grants
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE oauth_tokens
  ALTER COLUMN grant_id TYPE text USING grant_id::text;
ALTER TABLE objects
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text;
ALTER TABLE participants
  ALTER COLUMN interaction_id TYPE text USING interaction_id::text,
  ALTER COLUMN handle_id TYPE text USING handle_id::text;
ALTER TABLE parts
  ALTER COLUMN interaction_id TYPE text USING interaction_id::text,
  ALTER COLUMN connection_id TYPE text USING connection_id::text,
  ALTER COLUMN author_handle_id TYPE text USING author_handle_id::text;
ALTER TABLE record_values
  ALTER COLUMN record_id TYPE text USING record_id::text,
  ALTER COLUMN attribute_id TYPE text USING attribute_id::text,
  ALTER COLUMN ref_record_id TYPE text USING ref_record_id::text,
  ALTER COLUMN actor_id TYPE text USING actor_id::text;
ALTER TABLE records
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN object_id TYPE text USING object_id::text;
ALTER TABLE saved_filters
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN object_id TYPE text USING object_id::text;
ALTER TABLE sessions
  ALTER COLUMN user_id TYPE text USING user_id::text;
ALTER TABLE sync_cursors
  ALTER COLUMN connection_id TYPE text USING connection_id::text;
ALTER TABLE users
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text;
ALTER TABLE workspace_invites
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN invited_by TYPE text USING invited_by::text;
ALTER TABLE workspace_knowledge_pages
  ALTER COLUMN workspace_id TYPE text USING workspace_id::text,
  ALTER COLUMN page_id TYPE text USING page_id::text;
ALTER TABLE workspaces
  ALTER COLUMN id DROP DEFAULT,
  ALTER COLUMN id TYPE text USING id::text,
  ALTER COLUMN company_page_id TYPE text USING company_page_id::text;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE attributes ADD CONSTRAINT attributes_object_id_fkey FOREIGN KEY (object_id) REFERENCES objects(id) ON DELETE CASCADE;
ALTER TABLE attributes ADD CONSTRAINT attributes_target_object_id_fkey FOREIGN KEY (target_object_id) REFERENCES objects(id) ON DELETE CASCADE;
ALTER TABLE connections ADD CONSTRAINT connections_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE connections ADD CONSTRAINT connections_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE domain_rules ADD CONSTRAINT domain_rules_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE gmail_drafts ADD CONSTRAINT gmail_drafts_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE;
ALTER TABLE gmail_drafts ADD CONSTRAINT gmail_drafts_follow_up_id_fkey FOREIGN KEY (follow_up_id) REFERENCES records(id) ON DELETE SET NULL;
ALTER TABLE handles ADD CONSTRAINT handles_person_id_fkey FOREIGN KEY (person_id) REFERENCES records(id) ON DELETE SET NULL;
ALTER TABLE handles ADD CONSTRAINT handles_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE identities ADD CONSTRAINT identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE interactions ADD CONSTRAINT interactions_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE SET NULL;
ALTER TABLE interactions ADD CONSTRAINT interactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE interactions ADD CONSTRAINT interactions_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE links ADD CONSTRAINT links_interaction_id_fkey FOREIGN KEY (interaction_id) REFERENCES interactions(id) ON DELETE CASCADE;
ALTER TABLE links ADD CONSTRAINT links_record_id_fkey FOREIGN KEY (record_id) REFERENCES records(id) ON DELETE CASCADE;
ALTER TABLE oauth_codes ADD CONSTRAINT oauth_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE oauth_grants ADD CONSTRAINT oauth_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE oauth_tokens ADD CONSTRAINT oauth_tokens_grant_id_fkey FOREIGN KEY (grant_id) REFERENCES oauth_grants(id) ON DELETE CASCADE;
ALTER TABLE objects ADD CONSTRAINT objects_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE participants ADD CONSTRAINT participants_handle_id_fkey FOREIGN KEY (handle_id) REFERENCES handles(id) ON DELETE CASCADE;
ALTER TABLE participants ADD CONSTRAINT participants_interaction_id_fkey FOREIGN KEY (interaction_id) REFERENCES interactions(id) ON DELETE CASCADE;
ALTER TABLE parts ADD CONSTRAINT parts_author_handle_id_fkey FOREIGN KEY (author_handle_id) REFERENCES handles(id) ON DELETE SET NULL;
ALTER TABLE parts ADD CONSTRAINT parts_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE SET NULL;
ALTER TABLE parts ADD CONSTRAINT parts_interaction_id_fkey FOREIGN KEY (interaction_id) REFERENCES interactions(id) ON DELETE CASCADE;
ALTER TABLE record_values ADD CONSTRAINT record_values_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE record_values ADD CONSTRAINT record_values_attribute_id_fkey FOREIGN KEY (attribute_id) REFERENCES attributes(id) ON DELETE CASCADE;
ALTER TABLE record_values ADD CONSTRAINT record_values_record_id_fkey FOREIGN KEY (record_id) REFERENCES records(id) ON DELETE CASCADE;
ALTER TABLE record_values ADD CONSTRAINT record_values_ref_record_id_fkey FOREIGN KEY (ref_record_id) REFERENCES records(id) ON DELETE CASCADE;
ALTER TABLE records ADD CONSTRAINT records_object_id_fkey FOREIGN KEY (object_id) REFERENCES objects(id) ON DELETE CASCADE;
ALTER TABLE records ADD CONSTRAINT records_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE saved_filters ADD CONSTRAINT saved_filters_object_id_fkey FOREIGN KEY (object_id) REFERENCES objects(id) ON DELETE CASCADE;
ALTER TABLE saved_filters ADD CONSTRAINT saved_filters_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE sessions ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE sync_cursors ADD CONSTRAINT sync_cursors_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE;
ALTER TABLE users ADD CONSTRAINT users_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE workspace_invites ADD CONSTRAINT workspace_invites_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE workspace_invites ADD CONSTRAINT workspace_invites_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE workspace_knowledge_pages ADD CONSTRAINT workspace_knowledge_pages_page_id_fkey FOREIGN KEY (page_id) REFERENCES records(id) ON DELETE CASCADE;
ALTER TABLE workspace_knowledge_pages ADD CONSTRAINT workspace_knowledge_pages_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_company_page_id_fkey FOREIGN KEY (company_page_id) REFERENCES records(id) ON DELETE SET NULL;

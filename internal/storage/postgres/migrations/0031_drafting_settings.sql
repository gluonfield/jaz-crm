-- +goose Up
ALTER TABLE workspaces ADD COLUMN drafting_web_access boolean NOT NULL DEFAULT false;

CREATE TABLE workspace_knowledge_pages (
  workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
  page_id uuid NOT NULL REFERENCES records ON DELETE CASCADE,
  position integer NOT NULL,
  PRIMARY KEY (workspace_id, page_id)
);

INSERT INTO workspace_knowledge_pages (workspace_id, page_id, position)
SELECT id, company_page_id, 1 FROM workspaces WHERE company_page_id IS NOT NULL;

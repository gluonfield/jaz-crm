-- +goose Up
UPDATE record_values AS draft
SET active_until = now()
FROM attributes AS field JOIN objects ON objects.id = field.object_id
WHERE draft.attribute_id = field.id AND field.slug = 'draft'
  AND objects.slug = 'follow_ups' AND draft.active_until IS NULL
  AND EXISTS (
    SELECT 1 FROM record_values AS state
    JOIN attributes AS status ON status.id = state.attribute_id
    WHERE state.record_id = draft.record_id AND state.active_until IS NULL
      AND state.text = 'Sent' AND status.slug = 'draft_status'
      AND status.object_id = objects.id
  );

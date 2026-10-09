-- +goose Up
-- People and companies keep their story in Context and timeline notes; the
-- Notes property is archived with its values, so a workspace can restore it.
UPDATE attributes SET archived = true FROM objects
WHERE objects.id = attributes.object_id AND objects.slug IN ('people', 'companies') AND attributes.slug = 'notes';

-- +goose Up
-- Webmail domains are shared by unrelated people, so a rule for one decided
-- strangers. Return the senders such a skip rule decided as they arrived,
-- which nobody saw, to triage, then forget the rules.
UPDATE handles SET triage = 'pending', decided_by = NULL, reason = ''
FROM domain_rules rule
WHERE rule.domain IN ('gmail.com', 'googlemail.com', 'outlook.com', 'hotmail.com', 'live.com', 'msn.com', 'yahoo.com', 'ymail.com',
    'icloud.com', 'me.com', 'mac.com', 'aol.com', 'proton.me', 'protonmail.com', 'gmx.com', 'gmx.de', 'web.de',
    'mail.com', 'yandex.com', 'yandex.ru', 'fastmail.com', 'hey.com', 'zoho.com', 'qq.com', '163.com')
  AND rule.triage = 'skipped' AND rule.workspace_id = handles.workspace_id
  AND handles.kind = 'email' AND split_part(handles.value, '@', 2) = rule.domain
  AND handles.triage = 'skipped' AND handles.decided_by = 'user' AND handles.reason = rule.reason
  AND handles.created_at >= rule.created_at;

DELETE FROM domain_rules
WHERE domain IN ('gmail.com', 'googlemail.com', 'outlook.com', 'hotmail.com', 'live.com', 'msn.com', 'yahoo.com', 'ymail.com',
    'icloud.com', 'me.com', 'mac.com', 'aol.com', 'proton.me', 'protonmail.com', 'gmx.com', 'gmx.de', 'web.de',
    'mail.com', 'yandex.com', 'yandex.ru', 'fastmail.com', 'hey.com', 'zoho.com', 'qq.com', '163.com');

-- +goose Down
SELECT 1;

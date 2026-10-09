# Interaction Channels

Augustinas found the interaction schema convoluted: `kind` and `source` both said email/gmail, there was no link to the original, and X conversations could not be represented properly. He approved this proposal on 2026-10-09.

- [x] `kind` is the shape: message, meeting, call or note. Email becomes a message on the email channel.
- [x] `channel` comes from one built-in list (Email, LinkedIn, WhatsApp, X, Telegram, SMS), required on messages and absent elsewhere.
- [x] Remove `source`; `connection_id` already says whose account a conversation synced from.
- [x] Add `url`, the original: Gmail thread, calendar event, LinkedIn or X thread, Granola note.
- [x] Remove `provenance`. URLs move to `url`; note citations move into the note text.
- [x] A handle is an address on a channel, so LinkedIn, X and Telegram chats get participants.
- [x] One channel list serves conversations, handles and the follow-up Channel field; follow-ups can use X.
- [x] People have an X profile field.
- [x] `url` identifies a logged conversation: logging the same thread again adds to it instead of creating another.
- [x] `log_interaction` can add a transcript or notes to an existing meeting.

## Findings

- Storage kept `kind=email` and rewrote it to `kind=message, channel=email` on every read and in the last-message query.
- Production on 2026-10-09: 9,768 Gmail threads, 217 calendar meetings, 2 manual meetings, 1 call, 7 LinkedIn chats, 52 notes; only email handles.
- 51 research notes keep their source citations in provenance, so those citations move into the note text rather than being dropped.
- Jim Mayer's 2 Oct LinkedIn exchange is two conversations sharing one thread URL. Both messages are date-only on the same day, so their order within the merged conversation cannot be recovered.

## Behaviour

- Logging never deletes: messages merge by side, day and opening words, so a full message replaces its preview and a preview never cuts a full message; participants and record links accumulate; a transcript or note body given anew replaces the old one.
- Handles store self-describing addresses: `linkedin.com/in/name`, `x.com/name`, `t.me/name`. A LinkedIn or X handle joins the person whose `linkedin_url` or `x_url` ends in that name; an unmatched one waits in triage, and keeping it creates a person with that profile.
- Gmail links open the thread in the mailbox it first synced from. Calendar links are Google's own `htmlLink`; the migration clears the calendar cursor so the next sync fetches them for existing meetings.

## Verification

- Full Go suite, vet and gofmt pass; frontend tests, typecheck, lint and both builds pass.
- New tests cover appending by url, preview upgrade without downgrade, unknown channels and non-link urls, X handles joining the profile owner, a stranger waiting in triage, a recorder adding to a synced meeting without changing it, and refusing hand-logged messages in a synced email thread. The migration test starts at version 47 and covers email links, calendar cursor reset, merging by thread link, skipped chats staying removed, re-logging against migrated message keys, note citations, recorded-meeting links, follow-up channel options and the X field.
- Negative controls fail as expected: without the preview guard, without profile matching, without deleting merged duplicates, with a different migration message key, and without the calendar link tag.
- Strict review: re-logging combines a conversation's values in `Log`, so the shared upsert stays plain for calendar sync (a renamed event keeps its new title); one rule keeps logged participants (record owner first, a new person only for an email or phone number); the core-property migration runs as a method on its property list; follow-up profile links render from one definition. Negative controls fail without the owner and new-person branches.
- Rehearsal: a read-only dump of production restored into scratch Postgres 18 migrated from 47 to 50 in about a second. 10,047 interactions became 10,046 (Jim's merge); every email thread has a Gmail link; 51 notes keep citations; every workspace has six channels and the X field. A scratch server on that copy showed the merged LinkedIn thread with Open in LinkedIn, an email thread with Open in Gmail, and the X link on a follow-up profile.

## Rollout

- `eab50ce` pushed to main; Railway Server and Worker report SUCCESS on `2b9c0f6`, a later commit that contains it.
- Read-only production checks: migration version 50; 9,768 email threads all carry a Gmail link; after the calendar resync, 215 of 217 synced meetings carry Google's event link and the other two are removed meetings; Jim Mayer's 2 Oct LinkedIn messages are one conversation with its thread link; every workspace has the X field. A production backup taken just before the push was deleted after these checks.

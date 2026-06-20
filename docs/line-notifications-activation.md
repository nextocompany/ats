# LINE Candidate Notifications — Activation Runbook (UAT #11)

> Candidate-facing LINE + email notifications for ATS lifecycle events. The push
> **transport is already built** (`backend/internal/notify/`): the real LINE
> Messaging-API push, the mock log-only default, and inline best-effort dispatch
> all ship today. UAT #11 added two missing notifications (interview **scheduled**
> with date/time/place/link, and a **hired → upload onboarding docs** CTA) and this
> runbook to flip the channel from `mock` to `real` on prod.
>
> CI/CD is billing-blocked → everything here is **operator-run `az`**. `NOTIFY_PROVIDER`
> defaults to `mock`, so the live api logs `[mock notify]` and sends nothing until you
> complete this runbook. Rollback is instant (`NOTIFY_PROVIDER=mock`).

## ⚠️ Read first — three things that silently break LINE push

1. **Same LINE provider, or the userId won't match.** We store the candidate's LINE
   **Login** `sub` (`candidates.line_user_id`, Login channel **2010375490**). The
   Messaging-API **push** addresses a `userId`. These are the **same value only when
   the Login channel and the Messaging-API channel (bot 2010375394) live under the
   SAME LINE provider** in the LINE Developers console. If they are in different
   providers, every push fails with `400 / "The property, 'to', in the request body
   is invalid"`. **Verify the provider linkage before flipping** — this is the #1
   cause of "it's real but nothing arrives". (Email still delivers regardless.)

2. **The candidate must have added the OA as a friend, or push returns 403.** Login
   is configured with `LINE_LOGIN_BOT_PROMPT=aggressive`, which offers "add the
   Official Account as a friend" during sign-in — but it is the user's choice. Push
   stays **best-effort**: a 403 is logged `non-fatal` and the email twin still
   reaches them. Nothing to do here except know that not every candidate gets the
   LINE message.

3. **`NOTIFY_LINE_TOKEN` is the Messaging-API channel access token — NOT the Login
   channel secret.** Issue a long-lived (or stateless) **channel access token** from
   the **Messaging API** channel (bot 2010375394) in the LINE console. Using
   `LINE_CHANNEL_SECRET` (the Login channel secret) here will 401 every push.

## What this controls

| Env (on `hrats-prod-api`) | Value | Effect |
|---|---|---|
| `NOTIFY_PROVIDER` | `real` | Switches `notify` from mock (log-only) to real LINE push + email |
| `NOTIFY_LINE_TOKEN` | `<Messaging-API channel access token>` | Bearer token for `POST api.line.me/v2/bot/message/push`. **Store as an ACA secret.** |
| `EMAIL_PROVIDER` | `real` (already set) | Email twin via ACS — already live from the membership rollout |
| `PORTAL_BASE_URL` | `https://<career-portal>` (already set) | Deep links in the copy (`/status`, `/account`) |

> **Boot guard:** setting `NOTIFY_PROVIDER=real` **without** `NOTIFY_LINE_TOKEN` makes
> `config.Load` fail fast (`config.go:428`) and the api will **not start**. Set both
> in the same `az containerapp update`.

## Steps

1. **Verify provider linkage** (LINE Developers console): confirm Login channel
   `2010375490` and Messaging-API channel (bot `2010375394`) are under **one provider**.
   If not, the candidate userIds won't match — stop and resolve with the LINE admin
   before continuing.

2. **Mint the Messaging-API channel access token** for bot `2010375394` and store it
   as an ACA secret:
   ```bash
   az containerapp secret set -n hrats-prod-api -g hrats-prod-rg \
     --secrets notify-line-token=<MESSAGING_API_CHANNEL_ACCESS_TOKEN>
   ```

3. **Flip the provider + bind the token** (single update so the boot guard is satisfied):
   ```bash
   az containerapp update -n hrats-prod-api -g hrats-prod-rg \
     --set-env-vars NOTIFY_PROVIDER=real NOTIFY_LINE_TOKEN=secretref:notify-line-token
   ```

4. **Confirm boot:** `curl -s https://<api>/health` → `200`. If the revision crash-loops,
   the token env is almost certainly missing (see boot guard).

## Smoke test

1. Pick a test candidate who has logged in via LINE (has `line_user_id`) **and** added
   the OA as a friend.
2. From the dashboard, **book an interview** for one of their applications (try both
   onsite and online).
3. Within seconds the candidate's LINE should show the dated message:
   `นัดสัมภาษณ์ … 📅 <date> เวลา <HH:MM> น. … 💻/📍 …`. Online → a Teams join link;
   onsite → the location text.
4. Set that application to **hired** → the LINE/email copy should contain
   `…/account` (upload onboarding documents).
5. **Logs:** `az containerapp logs show -n hrats-prod-api -g hrats-prod-rg --tail 50`
   — you should see real sends (no `[mock notify]`) and **no** 4xx from `api.line.me`.
   A `403` means the candidate isn't a friend of the OA (expected, non-fatal); a
   `400 'to' invalid` means the provider-linkage check (step 1) was not actually
   satisfied.

## Rollback

```bash
az containerapp update -n hrats-prod-api -g hrats-prod-rg --set-env-vars NOTIFY_PROVIDER=mock
```
Instant — reverts to log-only, no image rebuild. The token secret can stay.

## Notes

- **Independent channels.** LINE and email are sent independently best-effort: a
  candidate with only one handle is still reached, and a failure on one never blocks
  the other or the HR action.
- **Calendar invite vs notification.** `GRAPH_PROVIDER` is `mock` on prod, so the
  Teams/Outlook calendar invite email does **not** fire for online interviews — the
  new email twin from this feature is the real coverage for those.
- **No queue.** Dispatch is inline (no asynq) and best-effort, matching every other
  notification path in the api.

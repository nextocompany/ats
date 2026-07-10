# LINE Login + Notifications — Provisioning Runbook (slice 2.3)

Flip real LINE Login (LIFF id-token, verified server-side) and real LINE push
notifications. Code ships behind flags (PRs #50 Phase A + #51 Phase B); this is the
deploy-time wiring. Default everywhere is **mock** — nothing leaves the system until
the flags below are flipped.

> **Golden rule:** the **LINE Login channel** (login + LIFF) and the **Messaging API
> channel** (push) MUST live under the **same LINE provider**. The LINE user id
> (`sub`) captured at login is only pushable by a bot in the same provider.

---

## 0. Prerequisites

- PRs #50 + #51 merged to `main`.
- **`career-portal/Dockerfile` must bake `NEXT_PUBLIC_LIFF_ID`** (NEXT_PUBLIC_* are
  compile-time). Confirm it has, alongside the existing `NEXT_PUBLIC_API_URL`:
  ```dockerfile
  ARG NEXT_PUBLIC_LIFF_ID
  ENV NEXT_PUBLIC_LIFF_ID=${NEXT_PUBLIC_LIFF_ID}
  ```
  If missing, add it before building (otherwise the build-arg is ignored and the
  portal stays on the dev stub).
- Existing channel: **Messaging API channel `2010375394`** already created (the bot).
- Run migration `000011` (adds `candidates.line_user_id`) — bundled in Phase A; the
  ACA migrate job / startup applies it.

---

## 1. LINE Developers Console — channels

Console: https://developers.line.biz/console/

### 1a. Messaging API channel (already exists: `2010375394`) → push token
- Open the channel → **Messaging API** tab → **Channel access token (long-lived)** →
  **Issue** → copy → this is `NOTIFY_LINE_TOKEN`.
- (The channel secret is NOT used by our flow — verify uses only the channel id.)

### 1b. LINE Login channel (create under the SAME provider) → `LINE_CHANNEL_ID` + LIFF
- Provider → **Create a new channel → LINE Login**.
- **Basic settings** → copy **Channel ID** → this is `LINE_CHANNEL_ID` (backend uses it
  as `client_id` when verifying the id-token; it must equal the LIFF app's login channel).
- **LIFF** tab → **Add**:
  - **Endpoint URL**:
    ```
    https://hrats-prod-portal.yellowmoss-b9b985f7.southeastasia.azurecontainerapps.io
    ```
  - **Size**: Full · **Scopes**: `openid`, `profile`
  - Add → copy the **LIFF ID** → this is `NEXT_PUBLIC_LIFF_ID`.

---

## 2. Env mapping

| Env | Where it comes from | Used by |
|---|---|---|
| `LINE_PROVIDER=real` | flag | api (verify) |
| `LINE_CHANNEL_ID` | LINE Login channel → Channel ID | api (verify `client_id`) |
| `NOTIFY_PROVIDER=real` | flag | api + worker (push) |
| `NOTIFY_LINE_TOKEN` | Messaging API → long-lived token | api + worker (push) |
| `NEXT_PUBLIC_LIFF_ID` | LIFF app → LIFF ID | career-portal (build-arg) |
| `PORTAL_BASE_URL` | portal origin (already set) | message links |

---

## 3. Wire app config (secrets + env)

`NOTIFY_LINE_TOKEN` is a secret. `LINE_CHANNEL_ID` is not sensitive but kept with the rest.

```bash
RG=hrats-prod-rg

# secret on the apps that push (api + worker)
for app in hrats-prod-api hrats-prod-worker; do
  az containerapp secret set -g $RG -n $app \
    --secrets notify-line-token="<LONG_LIVED_TOKEN>"
  az containerapp update -g $RG -n $app --set-env-vars \
    NOTIFY_PROVIDER=real \
    NOTIFY_LINE_TOKEN=secretref:notify-line-token
done

# LINE Login verify (api only — the public apply endpoint lives there)
az containerapp update -g $RG -n hrats-prod-api --set-env-vars \
  LINE_PROVIDER=real \
  LINE_CHANNEL_ID="<LOGIN_CHANNEL_ID>"
```

> Config is fail-fast: `LINE_PROVIDER=real` requires `LINE_CHANNEL_ID`;
> `NOTIFY_PROVIDER=real` requires `NOTIFY_LINE_TOKEN`. A missing value crashes
> startup (by design).

---

## 4. Build + deploy

The portal must be **rebuilt** (LIFF id is compile-time baked). api/worker just roll.

```bash
ACR=hratsacr7qmhyxfjdyyl2
TAG=$(git rev-parse --short HEAD)   # on main, after #50/#51 merge

# career-portal — bake the LIFF id (AND keep the existing API url)
az acr build -r $ACR -t hr-ats/career-portal:$TAG \
  --build-arg NEXT_PUBLIC_API_URL=https://hrats-prod-api.yellowmoss-b9b985f7.southeastasia.azurecontainerapps.io \
  --build-arg NEXT_PUBLIC_LIFF_ID=<LIFF_ID> \
  career-portal
az containerapp update -g $RG -n hrats-prod-portal \
  --image $ACR.azurecr.io/hr-ats/career-portal:$TAG

# api + worker (notify + persist live in both)
az acr build -r $ACR -t hr-ats/api:$TAG    --build-arg SVC=api    backend
az acr build -r $ACR -t hr-ats/worker:$TAG --build-arg SVC=worker backend
az containerapp update -g $RG -n hrats-prod-api    --image $ACR.azurecr.io/hr-ats/api:$TAG
az containerapp update -g $RG -n hrats-prod-worker --image $ACR.azurecr.io/hr-ats/worker:$TAG
```

---

## 5. Smoke test (staging/pilot first)

1. **Friend the bot** — scan the Messaging channel's QR (console → Messaging API). Push
   only reaches users who added the bot.
2. **Apply from the LINE in-app browser** — open the portal in LINE → apply → LIFF login
   succeeds (no stub). Confirm in DB:
   ```sql
   SELECT full_name, line_user_id FROM candidates ORDER BY created_at DESC LIMIT 1;
   -- line_user_id should be a real U... id (not "U-dev-...")
   ```
3. **Status push** — move that application to shortlisted/interview/hired (HR dashboard)
   → a real LINE message arrives.
4. **Re-engagement push** — open a matching vacancy → candidate with a `line_user_id`
   gets the "new job" message.
5. **Verify the id-token aud** — a wrong `LINE_CHANNEL_ID` makes verify 4xx → apply 401.

---

## Rollback

Flip flags back to mock (no redeploy of code needed):
```bash
az containerapp update -g hrats-prod-rg -n hrats-prod-api    --set-env-vars LINE_PROVIDER=mock NOTIFY_PROVIDER=mock
az containerapp update -g hrats-prod-rg -n hrats-prod-worker --set-env-vars NOTIFY_PROVIDER=mock
```
The portal will still attempt LIFF if it was rebuilt with a LIFF id; to fully revert
the portal, rebuild without `NEXT_PUBLIC_LIFF_ID` (falls back to the stub).

---

## Notes / gotchas

- **Same provider** for Login + Messaging — otherwise `sub` is not pushable (the #1
  failure mode).
- **Friend requirement** — a candidate who logs in but never friends the bot has a valid
  `sub` but push returns an error. Our code is best-effort: it logs and never fails the
  apply or status change. Consider an "add friend" CTA on the portal later.
- **Email is NOT wired** — `NOTIFY_EMAIL_FROM` is read but not fail-fast; the email
  channel returns an error (stub). LINE push only in this slice.
- **Demo/legacy candidates** have no `line_user_id` → they silently receive no push
  (expected).
- **Secrets exposed during setup** (channel secret / any token pasted in chat or logs)
  must be **reissued** in the console.
- **`PORTAL_BASE_URL`** must point at the public portal so message links resolve.
```

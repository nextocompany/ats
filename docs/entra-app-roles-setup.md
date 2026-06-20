# Entra App Roles Setup (UAT #6 RBAC)

Backend verifies `aud == AZURE_AD_CLIENT_ID` and reads the role from the token's
`roles` claim (`internal/auth/entra.go:54`). There is **no JIT provisioning** for
Entra users: the role is whatever app role Azure puts in the token. An unassigned
user gets `role=""` → RBAC fails closed to store scope (near-zero permissions).

The matrix editor in the dashboard manages the **DB role→permission matrix** (what
each role *can do*). Azure app roles only decide **which role string** an Entra user
receives. Both must agree on the 7 role keys below.

## Target app registration

| Field | Value |
|---|---|
| App (client) ID | `57c7d338-47be-4726-bec5-560853620d1f` |
| Tenant ID | `aaabefb4-0433-494c-b50e-67b0f4b5f05c` |
| `signInAudience` | `AzureADMultipleOrgs` (multi-tenant, keep as-is) |

App roles MUST be defined on **this** registration (it is the token audience the API
checks). Token `roles` claim value must equal the RBAC role key exactly.

## The 7 role keys (must match RBAC seed, migration 000028)

| `value` (claim) | Display name | Default scope |
|---|---|---|
| `super_admin` | Super admin | all (code bypass: always allowed) |
| `regional_director` | Regional director | all |
| `auditor` | Auditor | all |
| `operation_director` | Operation director | subregion |
| `sgm` | Store GM | store |
| `hr_manager` | HR manager | store |
| `hr_staff` | HR staff | store |

## Method A — App roles manifest (recommended, all 7 at once)

Azure Portal → Microsoft Entra ID → App registrations → (the app above) →
**App roles** → **Create app role** (or *Manifest* tab and paste the `appRoles`
array). Each `id` GUID is pre-generated and unique; do not reuse across apps.

```json
"appRoles": [
  {
    "allowedMemberTypes": ["User"],
    "displayName": "Super admin",
    "description": "Full system administration; bypasses RBAC.",
    "value": "super_admin",
    "id": "b42a8533-adf8-4e89-878a-20b3a9b23924",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "Regional director",
    "description": "Company-wide visibility; executive + reports.",
    "value": "regional_director",
    "id": "3d19a9d5-2ef9-4f24-aafb-d1fd0fc231da",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "Auditor",
    "description": "Read-only company-wide audit access.",
    "value": "auditor",
    "id": "732c3343-4a27-4935-b9f9-d649cf7d048a",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "Operation director",
    "description": "Subregion-scoped operations oversight.",
    "value": "operation_director",
    "id": "d109d8f3-c64e-4ba6-a7ec-4dd5db57270d",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "Store GM",
    "description": "Store-scoped general manager.",
    "value": "sgm",
    "id": "0baaa830-6ca1-4a90-95bd-331e8682ed0c",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "HR manager",
    "description": "Store-scoped HR manager.",
    "value": "hr_manager",
    "id": "8287c7e3-2e7f-45d1-8a68-23ac22e5ebe1",
    "isEnabled": true
  },
  {
    "allowedMemberTypes": ["User"],
    "displayName": "HR staff",
    "description": "Store-scoped HR staff.",
    "value": "hr_staff",
    "id": "2187f674-3530-46cd-8237-8aba42095f8c",
    "isEnabled": true
  }
]
```

## Method B — Portal UI (one role at a time)

For each of the 7 roles: App registration → **App roles** → **Create app role** →
- Display name: from the table
- Allowed member types: **Users/Groups**
- Value: the exact key (e.g. `hr_manager`) — **no spaces, case-sensitive**
- Description: any
- Enable this app role: ✔

## Assign roles to users (Enterprise app)

App roles are *defined* on the registration but *assigned* on the matching
**Enterprise application**:

Entra ID → **Enterprise applications** → (same app) → **Users and groups** →
**Add user/group** → pick the user → pick the **Role** → Assign.

Recommended: same blade → **Properties** → **Assignment required? = Yes** so only
assigned users can sign in.

## Custom claims for scoped roles (store_id / subregion)

Store-scoped roles (`sgm`, `hr_manager`, `hr_staff`) need a `store_id` claim and
`operation_director` needs a `subregion` claim, or scope resolution sees nothing
(a store-scoped user with no store sees an empty UI — this is fail-closed, not a
bug). Roles with scope `all` (`super_admin`, `regional_director`, `auditor`) need
**no** extra claims, so do the first UAT pass with those.

### Why directory extensions (not a short `store_id` claim)

The cleanest token claim would be a short name `store_id`, but that requires a
**claims-mapping policy**, and a multi-tenant app (`AzureADMultipleOrgs`) cannot
emit mapped claims without a **custom signing key** (Azure blocks
`acceptMappedClaims` for multi-tenant). So the supported path is a **directory
extension attribute**, which surfaces as a prefixed, string-typed claim:

```
extension_57c7d33847be4726bec5560853620d1f_store_id   (string)
extension_57c7d33847be4726bec5560853620d1f_subregion  (string)
```

The backend accepts **both** the short name and this prefixed form, and parses the
string to an int (`internal/auth/entra_scope_claims.go`). No custom signing key
needed.

### Steps

1. **Register the extension attributes** (Graph, once). On the app object
   (`57c7d338…`):

   ```http
   POST https://graph.microsoft.com/v1.0/applications/{object-id}/extensionProperties
   { "name": "store_id",   "dataType": "String", "targetObjects": ["User"] }

   POST https://graph.microsoft.com/v1.0/applications/{object-id}/extensionProperties
   { "name": "subregion",  "dataType": "String", "targetObjects": ["User"] }
   ```

   Graph returns the full claim name, e.g. `extension_57c7d338…_store_id`. (Use the
   app **object id**, not the client id, in the URL.)

2. **Set per-user values** (Graph, per assigned user):

   ```http
   PATCH https://graph.microsoft.com/v1.0/users/{user-id}
   {
     "extension_57c7d33847be4726bec5560853620d1f_store_id": "12",
     "extension_57c7d33847be4726bec5560853620d1f_subregion": "East"
   }
   ```

   Set `store_id` for store-scoped users; set `subregion` for
   `operation_director`. Values are strings; the backend converts `store_id` to int.

3. **Add the optional claims** so they ride in the token. App registration →
   **Token configuration** → **Add optional claim** → token type **ID** (and
   **Access** if the dashboard sends an access token to the API) → there is no
   built-in entry, so use **Add optional claim → … → directory extension** and pick
   the two `extension_…_store_id` / `_subregion` attributes registered in step 1.

4. **Re-issue the token** (sign out / in) and confirm at <https://jwt.ms> that the
   token carries the `extension_…_store_id` / `_subregion` claims with the per-user
   values.

> Requires the backend build that resolves these claims
> (`internal/auth/entra_scope_claims.go`). Roll the api before testing scoped roles.

## Verify

1. Sign in to the dashboard with an assigned test user.
2. Decode the access/ID token at <https://jwt.ms> and confirm a `roles` claim with
   the expected key (e.g. `"roles": ["hr_manager"]`).
3. Confirm the dashboard gates features per that role; `super_admin` sees everything,
   an **unassigned** user fails closed (store scope, near-empty UI).

## Graph API commands (az rest, copy-paste)

Concrete values for this tenant. The signed-in az user needs rights to write the
app + user objects (Application.ReadWrite.All / Directory.ReadWrite.All or be an
app owner + user admin). A `403` means the account lacks the directory role.

```bash
# --- constants (this tenant) ---
APP_OBJECT_ID=88f1e969-9de1-4d5a-87cc-fea0bd8b3be9          # HR ATS Dashboard (object id, NOT client id)
EXT=extension_57c7d33847be4726bec5560853620d1f             # client id, dashes stripped
```

### 1. Register the two directory extension attributes (once)

```bash
az rest --method POST \
  --url "https://graph.microsoft.com/v1.0/applications/$APP_OBJECT_ID/extensionProperties" \
  --headers "Content-Type=application/json" \
  --body '{"name":"store_id","dataType":"String","targetObjects":["User"]}'

az rest --method POST \
  --url "https://graph.microsoft.com/v1.0/applications/$APP_OBJECT_ID/extensionProperties" \
  --headers "Content-Type=application/json" \
  --body '{"name":"subregion","dataType":"String","targetObjects":["User"]}'
```

Each response `name` is the full claim name, e.g.
`extension_57c7d33847be4726bec5560853620d1f_store_id`. Confirm it matches `$EXT_store_id`.

### 2. Set per-user values (per assigned user)

`store_id` for store-scoped roles (`sgm`/`hr_manager`/`hr_staff`); `subregion` for
`operation_director`. Values are strings; the backend converts `store_id` to int.

```bash
USER="firstname.lastname@ert.co.th"   # UPN or user object id

az rest --method PATCH \
  --url "https://graph.microsoft.com/v1.0/users/$USER" \
  --headers "Content-Type=application/json" \
  --body "{\"${EXT}_store_id\":\"12\",\"${EXT}_subregion\":\"East\"}"
```

### 3. Verify the stored values

```bash
az rest --method GET \
  --url "https://graph.microsoft.com/v1.0/users/$USER?\$select=displayName,${EXT}_store_id,${EXT}_subregion"
```

### 4. Surface them as optional claims (so they ride in the token)

Registering + setting an extension does NOT put it in the token; it must also be an
optional claim. **Portal is the reliable path:** App registration → **Token
configuration** → **Add optional claim** → ID (and Access) → **directory extension**
→ pick `store_id` + `subregion`.

Graph alternative (merge into existing `optionalClaims`, do not blind-overwrite):

```bash
az rest --method PATCH \
  --url "https://graph.microsoft.com/v1.0/applications/$APP_OBJECT_ID" \
  --headers "Content-Type=application/json" \
  --body "{\"optionalClaims\":{\"idToken\":[
    {\"name\":\"${EXT}_store_id\",\"source\":\"user\",\"essential\":false},
    {\"name\":\"${EXT}_subregion\",\"source\":\"user\",\"essential\":false}],
    \"accessToken\":[
    {\"name\":\"${EXT}_store_id\",\"source\":\"user\",\"essential\":false},
    {\"name\":\"${EXT}_subregion\",\"source\":\"user\",\"essential\":false}]}}"
```

Then sign out / in and decode the token at <https://jwt.ms> to confirm the
`extension_…_store_id` / `_subregion` claims carry the per-user values.

## UAT #6 gotchas

- Role string mismatch (typo / wrong case) → backend can't find it → fail closed to
  store scope. Looks like "RBAC broken" but it is an Azure app-role naming issue.
- Token caches the old roles; sign out / fresh token after assigning.
- New app roles can take a few minutes to propagate; re-issue the token.
- Editing the DB matrix in the dashboard does **not** change an Entra user's role —
  that only comes from the Azure app-role assignment.

# Azure AI Search — Provisioning Runbook (seam 2.4)

Provision the candidate search index backing `AI_SEARCH_PROVIDER=azure`. Mirrors
`docs/azure-openai-provisioning.md`. The app uses the **admin** api-key for both
index population (push) and query.

## 1. Create the service

```bash
RG=hrats-prod-rg
LOC=southeastasia            # AI Search is broadly available; fall back to a nearby region if unavailable
SEARCH=hrats-prod-search     # globally-unique

az search service create \
  -g "$RG" -n "$SEARCH" --sku free \
  --location "$LOC" --partition-count 1 --replica-count 1
```

- **Free** SKU: 1 partition, 50 MB, 3 indexes, no cost — fits the pilot roster.
  Use `--sku basic` (~paid) for production headroom / scale.
- The **index is created by the app** (`EnsureIndex` / `cmd/reindex`), not here.

## 2. Get the admin key + endpoint

```bash
az search admin-key show -g "$RG" --service-name "$SEARCH" --query primaryKey -o tsv
echo "https://$SEARCH.search.windows.net"
```

> A **query** key (`az search query-key list`) is read-only and will 403 on index
> create/push. Always wire the **admin** key as `AZURE_SEARCH_KEY`.

## 3. Wire app config (api + worker)

Set on `hrats-prod-api` **and** `hrats-prod-worker` (worker keeps the index fresh
from the pipeline). Endpoint/index as plain env; key as a secret.

```bash
# secret (both apps)
for APP in hrats-prod-api hrats-prod-worker; do
  az containerapp secret set -g "$RG" -n "$APP" --secrets search-key="<admin-key>"
  az containerapp update  -g "$RG" -n "$APP" \
    --set-env-vars AI_SEARCH_PROVIDER=azure \
                   AZURE_SEARCH_ENDPOINT="https://$SEARCH.search.windows.net" \
                   AZURE_SEARCH_INDEX=candidates \
                   AZURE_SEARCH_KEY=secretref:search-key
done
```

> **Gotcha:** a secret-only change does NOT roll a revision — the
> `--set-env-vars` above forces one. If you change only the secret value later,
> `az containerapp revision restart`. (See infra gotchas in the deploy notes.)

## 4. Create the index + backfill

`cmd/reindex` ensures the index schema (Thai `th.microsoft` analyzer) and pushes
every candidate-with-an-application. Run **after** migrations + data seed.

- **As an ACA Job** (preferred, OIDC/CD):
  ```bash
  az containerapp job create -g "$RG" -n hrats-prod-reindex \
    --environment <aca-env> --trigger-type Manual \
    --image <acr>/hr-ats/reindex:<tag> \
    --secrets search-key=<admin-key> db-url=<db-url> \
    --env-vars AI_SEARCH_PROVIDER=azure AZURE_SEARCH_ENDPOINT=... AZURE_SEARCH_INDEX=candidates \
               AZURE_SEARCH_KEY=secretref:search-key DB_URL=secretref:db-url
  az containerapp job start -g "$RG" -n hrats-prod-reindex
  ```
  (Build a `hr-ats/reindex` image with `--build-arg SVC=reindex` if the backend
  Dockerfile is multi-cmd, or a dedicated Dockerfile target.)
- **Or operator one-off** from a machine with DB + Search access:
  ```bash
  AI_SEARCH_PROVIDER=azure AZURE_SEARCH_ENDPOINT=... AZURE_SEARCH_KEY=<admin> \
  AZURE_SEARCH_INDEX=candidates DB_URL=<db-url> go run ./cmd/reindex
  ```

## 5. Smoke test

```bash
EP=https://$SEARCH.search.windows.net ; KEY=<admin-key>
# index exists + doc count > 0
curl -s -H "api-key: $KEY" "$EP/indexes/candidates?api-version=2024-07-01" | jq '.fields[].name'
curl -s -H "api-key: $KEY" "$EP/indexes/candidates/docs/\$count?api-version=2024-07-01"
# query through the app (needs an HR JWT) → results come from Azure now
curl -s "https://hrats-prod-api.<domain>/api/v1/candidates/search?q=<thai-name>" -H "Authorization: Bearer <jwt>"
```

## Rollback

Instant: `AI_SEARCH_PROVIDER=mock` (+ revision restart) → back to the Postgres
trigram baseline. No data loss; re-flip to `azure` anytime (index persists).

## Notes

- **Region availability:** if `southeastasia` rejects the SKU/service, pick a
  nearby region (the endpoint is region-independent in app config) — same pattern
  as the OpenAI eastus workaround.
- **Keeping fresh:** the worker re-indexes a candidate after each scoring; the api
  re-indexes on bulk status changes. Other mutations (e.g. PDPA delete) are not
  yet hooked — re-run `cmd/reindex` periodically as a safety re-sync, or add a
  delete hook in a follow-up.
- **Schema change:** `mergeOrUpload` is additive; a breaking field change needs a
  manual `DELETE /indexes/candidates` then re-run `cmd/reindex`.
- **`cmd/reindex` config is strict:** `config.Load()` validates the full app
  config, so the job/run must also have `REDIS_URL`, `AZURE_BLOB_CONNECTION_STRING`,
  and `JWT_SECRET` set (it doesn't *use* them). An ACA Job that inherits the api's
  secrets satisfies this for free; an operator one-off must export them too.
- **Switching the index between environments:** the index is keyed by
  `candidate_id`, which differs per DB. Before backfilling a new environment into
  an index that held another env's data, `DELETE /indexes/candidates` first (or
  the old docs linger as orphans).

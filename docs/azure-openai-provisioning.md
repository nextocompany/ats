# Azure OpenAI — Provisioning Runbook (ATS CV Parser)

> Parallel track started 2026-06-03. Goal: provision Azure OpenAI early because
> access/quota can have lead time. **Not yet wired into runtime** — the backend
> stays on `AI_PROVIDER=mock` until both Azure OpenAI *and* Document Intelligence
> are ready (see caveat below). This doc is a checklist for the Azure portal/CLI.

## ⚠️ Read first — OpenAI and Doc Intelligence are bundled

`backend/internal/ai/factory.go` flips **both** clients together:

```go
if cfg.UsesAzureAI() {            // AI_PROVIDER=azure
    return NewAzureOCR(DocIntelEndpoint, DocIntelKey),
           NewAzureParser(OpenAIEndpoint, OpenAIKey, OpenAIDeployment)
}
return NewMockOCR(), NewMockParser()
```

And `pkg/config/config.go` (Load) **rejects startup** if `AI_PROVIDER=azure` and either
the OpenAI *or* Doc Intelligence keys are missing. So:

- You **cannot** run "half azure". Provision **Azure OpenAI now** (long lead), but keep
  `AI_PROVIDER=mock` until **Azure AI Document Intelligence** is also provisioned.
- Provisioning OpenAI early does **not** change app behavior — safe to do in parallel.

## What the code expects (do not deviate)

From `backend/internal/ai/azure_parser.go`:

| Thing | Value the code uses | Notes |
|---|---|---|
| API version | `2024-08-01-preview` | `openAIAPIVersion` const |
| Endpoint shape | `POST {endpoint}/openai/deployments/{deployment}/chat/completions?api-version=2024-08-01-preview` | `{endpoint}` = resource endpoint, no trailing slash |
| Auth | header `api-key: <KEY>` | resource key (not Entra token) |
| Response format | `response_format: {type: "json_object"}` | **deployment model must support JSON mode** |
| Deployment name | env `AZURE_OPENAI_DEPLOYMENT`, default **`hr-screening-gpt4o`** | name your deployment this, or override the env var |

→ Deploy a **GPT-4o** model (JSON mode + chat completions). Name the deployment
`hr-screening-gpt4o` to match the default, or set `AZURE_OPENAI_DEPLOYMENT` to whatever you name it.

## Steps (portal)

1. **Subscription check** — ensure the subscription has **Azure OpenAI** enabled. If the
   resource type isn't available, submit the Azure OpenAI access request and wait for approval
   (this is the lead-time item — do it first).
2. **Create the resource** — *Azure AI services → Azure OpenAI → Create*.
   - Region: pick one that offers **GPT-4o** with chat completions + JSON mode (e.g. East US / Sweden Central — confirm current availability).
   - Pricing tier: Standard (S0).
   - Note the **Endpoint** (e.g. `https://<name>.openai.azure.com/`) and **Key 1**.
3. **Deploy the model** — *Azure AI Foundry / OpenAI Studio → Deployments → Create new deployment*.
   - Model: **gpt-4o** (a version supporting `2024-08-01-preview` + JSON mode).
   - Deployment name: **`hr-screening-gpt4o`**.
   - Set a **TPM quota** large enough for CV-parsing batches; request a quota increase if the default is too low (also a lead-time item).
4. **Record secrets** (do not commit) — Endpoint, Key, Deployment name.

### Or via CLI (reference)

```bash
az cognitiveservices account create \
  --name <openai-name> --resource-group <rg> --kind OpenAI \
  --sku S0 --location <region> --yes
az cognitiveservices account deployment create \
  --name <openai-name> --resource-group <rg> \
  --deployment-name hr-screening-gpt4o \
  --model-name gpt-4o --model-version <ver> --model-format OpenAI \
  --sku-capacity <tpm> --sku-name Standard
az cognitiveservices account show   --name <openai-name> -g <rg> --query properties.endpoint -o tsv
az cognitiveservices account keys list --name <openai-name> -g <rg> --query key1 -o tsv
```

## Env mapping (backend `.env`)

From `backend/.env.example` lines 28-32 — fill these (leave `AI_PROVIDER=mock` for now):

```dotenv
AI_PROVIDER=mock                 # keep mock until Doc Intelligence is also ready
AZURE_OPENAI_ENDPOINT=https://<name>.openai.azure.com
AZURE_OPENAI_KEY=<key1>
AZURE_OPENAI_DEPLOYMENT=hr-screening-gpt4o
# Still required to flip AI_PROVIDER=azure (separate resource, provision next):
AZURE_DOC_INTEL_ENDPOINT=
AZURE_DOC_INTEL_KEY=
```

## Smoke test (once OpenAI key is in hand — optional, before Doc Intel exists)

The app can't be flipped to `azure` yet (needs Doc Intel too), but you can verify the
OpenAI endpoint/deployment/key independently with a raw call that mirrors what
`azure_parser.go` sends:

```bash
curl -s -X POST \
  "$AZURE_OPENAI_ENDPOINT/openai/deployments/$AZURE_OPENAI_DEPLOYMENT/chat/completions?api-version=2024-08-01-preview" \
  -H "api-key: $AZURE_OPENAI_KEY" -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"Return {\"ok\":true} as JSON"}],"temperature":0,"max_tokens":20,"response_format":{"type":"json_object"}}'
```

EXPECT: HTTP 200 with a JSON `choices[].message.content`. A 404 = wrong deployment name;
401 = wrong key; 400 on `response_format` = model/api-version doesn't support JSON mode.

## Next (separate, when ready)

- Provision **Azure AI Document Intelligence** (`prebuilt-layout`, api-version `2024-11-30` — see `azure_ocr.go`); fill `AZURE_DOC_INTEL_*`.
- Then set `AI_PROVIDER=azure` and run the integration path. Targeted for **S8 / UAT**, or an earlier real-Azure smoke slice if desired.
- Entra ID (`AUTH_PROVIDER=real`) and a real Storage Account are separate tracks, also S8-era.

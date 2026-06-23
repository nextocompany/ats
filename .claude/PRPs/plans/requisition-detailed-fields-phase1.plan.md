# Plan: Requisition Detailed Fields - Phase 1 (Form + Backend)

## Summary
Expand the manual Requisition feature so HR can capture a full, standardized job description per opening: Responsibilities, Qualifications, Benefits, Other, plus Employment Type, Salary range, Priority, and Reason for opening. All new fields are stored on the shared `vacancies` table (requisitions are `source='manual'` vacancy rows). Responsibilities + Qualifications + Benefits are **prefilled from the linked position catalog** when HR picks a position, then editable. To make Benefits prefillable (and portal-surfaceable in Phase 2), this phase also adds a `benefits TEXT` column to the `positions` catalog table. Position, Store, and Headcount already exist and are unchanged.

> **Key behavior to communicate in the UI:** the portal (Phase 2) is position-keyed, so what candidates see is the POSITION catalog's JD, NOT the per-requisition edits. The requisition JD copies (responsibilities/qualifications/benefits) are the standard JD captured into the approval record; HR edits there do NOT publish to the portal. Phase 1 must label these fields so HR is not misled into thinking edits go live.

## User Story
As an HR user opening a new requisition,
I want to fill in a complete, structured job description (responsibilities, qualifications, benefits, employment type, salary, urgency, reason),
So that every opening is documented to a consistent standard and is ready to publish to candidates.

## Problem → Solution
Today a requisition captures only Position + Store + Headcount (3 fields) → a requisition captures a full JD with sensible prefill from the position catalog and opening-specific metadata (employment type, salary, priority, reason).

## Metadata
- **Complexity**: Medium
- **Source PRD**: N/A (free-form request via /prp-plan)
- **PRD Phase**: N/A
- **Estimated Files**: ~10 (1 migration pair, 4 backend, 4 frontend, 2 i18n)
- **Next migration number**: `000040` (latest existing is `000039_backfill_candidate_accounts`)

---

## UX Design

### Before
```
┌─ New requisition ───────────────┐
│ Position   [ Select ▾ ]         │
│ Store      [ Select ▾ ]         │
│ Headcount  [  1  ]              │
│                  [Cancel][Create]│
└─────────────────────────────────┘
```

### After
```
┌─ New requisition ───────────────────────────────┐  (sm:max-w-2xl, body scrolls)
│ Position        [ Select ▾ ]  ← on pick: prefill │
│ Store           [ Select ▾ ]                      │
│ Headcount [ 1 ]   Employment type [ Full-time ▾ ] │
│ Salary  [min]  -  [max]   (blank = ตามตกลง)       │
│ Priority [ Normal ▾ ]   Reason [ New headcount ▾ ]│
│ Responsibilities  [ textarea, prefilled ]         │
│ Qualifications    [ textarea, prefilled ]         │
│ Benefits          [ textarea ]                    │
│ Other             [ textarea ]                    │
│                              [Cancel][Create]     │
└───────────────────────────────────────────────────┘
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Pick a position | Only sets position_id | Also fetches position detail and prefills empty Responsibilities + Qualifications textareas | Create mode only; never overwrites text the user already typed; edit mode does not auto-prefill |
| Submit body | `{position_id, store_id, headcount}` | adds `responsibilities, qualifications, benefits, other_details, employment_type, salary_min, salary_max, priority, open_reason` | All new fields optional |
| Dialog width | `sm:max-w-md` | `sm:max-w-2xl` + scrollable body | Accommodates 4 textareas + selects |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/requisitions/repository.go` | 18-26, 106-151 | `reqColumns`/`reqFrom`, **positional** scan, Create INSERT, Update sparse-SET - every new column touches these |
| P0 | `backend/internal/requisitions/model.go` | 21-102 | Status consts, `Requisition`, `CreateInput`, `UpdateInput`, `ListFilter`, repo interface |
| P0 | `backend/internal/requisitions/handler.go` | 16, 31-39, 88-120, 124-160, 236-246 | `maxHeadcount`, routes, `createReq`/`updateReq` DTOs + validation + `writeErr` |
| P0 | `backend/migrations/000029_requisitions.up.sql` | 1-47 | Pattern for ALTER vacancies + nullable/defaulted columns + RBAC grants |
| P0 | `frontend/components/requisitions/RequisitionDialog.tsx` | all (163) | The form to extend; Select function-child pattern, plain useState, submit |
| P0 | `frontend/lib/queries.ts` | 194-199, 809-868 | `usePositions`, requisition hooks; where to add `usePosition(id)` |
| P0 | `frontend/lib/types.ts` | 277-314 | `Position`, `Requisition`, `RequisitionInput`, `RequisitionFilter` to extend |
| P1 | `backend/internal/positions/handler.go` | 14-18, 35-51 | Positions routes (currently LIST only); add `GET /:id` here |
| P1 | `backend/internal/positions/model.go` | 21-31, 63-86 | `Position` struct + `FindByID` (already SELECTs responsibilities + qualifications) |
| P1 | `frontend/components/resume/OfferPanel.tsx` | 147-153 | Canonical raw `<textarea>` pattern to mirror (no Textarea UI component exists) |
| P1 | `frontend/components/requisitions/RequisitionTable.tsx` | all (115) | Optional priority badge; status label `status_${x}` pattern |
| P2 | `backend/internal/requisitions/handler_test.go` | 92-180 | Test harness (`stubRepo`, `fakeReader`) to extend with validation cases |
| P2 | `backend/internal/vacancies/model.go` | Upsert/`SetStatusByPSID` | Confirm PS sync uses explicit column list → new columns must stay nullable/defaulted |

## External Documentation
No external research needed - feature uses established internal patterns (pgx repo, Fiber handler, base-ui Select, next-intl, TanStack Query).

---

## Patterns to Mirror

### NAMING / MODEL STRUCT (no db tags; positional scan)
```go
// SOURCE: backend/internal/requisitions/model.go:38-53
type Requisition struct {
	ID            uuid.UUID  `json:"id"`
	PositionID    *uuid.UUID `json:"position_id"`
	PositionTitle string     `json:"position_title"`
	StoreID       *int       `json:"store_id"`
	StoreName     string     `json:"store_name"`
	Subregion     string     `json:"subregion"`
	Headcount     int        `json:"headcount"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	CreatedBy     *uuid.UUID `json:"created_by"`
	ApprovedBy    *uuid.UUID `json:"approved_by"`
	ApprovedAt    *time.Time `json:"approved_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
```

### REPOSITORY PROJECTION + POSITIONAL SCAN (the critical alignment)
```go
// SOURCE: backend/internal/requisitions/repository.go:18-26
const reqColumns = `
	v.id, v.position_id, COALESCE(NULLIF(p.title_en,''), p.title_th, '') AS position_title,
	v.store_id, COALESCE(s.store_name,'') AS store_name, COALESCE(s.subregion,'') AS subregion,
	v.headcount, v.status, v.source, v.created_by, v.approved_by, v.approved_at, v.created_at, v.updated_at`
const reqFrom = `
	FROM vacancies v
	LEFT JOIN positions p ON p.id = v.position_id
	LEFT JOIN stores s ON s.store_no = v.store_id`
// scanRequisition (same file) scans these columns POSITIONALLY in the exact same order.
// RULE: append every new column at the END of reqColumns AND at the END of the scan arg list, in identical order.
```

### REPOSITORY CREATE (INSERT + RETURNING id, then getByID)
```go
// SOURCE: backend/internal/requisitions/repository.go:106-116
const ins = `
	INSERT INTO vacancies (position_id, store_id, headcount, status, source, created_by, opened_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, NULL, now(), now())
	RETURNING id`
```

### REPOSITORY UPDATE (sparse SET builder)
```go
// SOURCE: backend/internal/requisitions/repository.go:118-151 (shape)
// Builds SET clauses only for non-nil pointer fields, then:
//   WHERE id=$N AND source='manual' AND status='pending_approval'
// 0 rows affected -> return ErrBadState
```

### HANDLER DTO + VALIDATION + ERROR MAP
```go
// SOURCE: backend/internal/requisitions/handler.go:88-116, 236-246
const maxHeadcount = 999 // handler.go:16
type createReq struct {
	PositionID string `json:"position_id"`
	StoreID    int    `json:"store_id"`
	Headcount  int    `json:"headcount"`
}
// validation: position_id parses as UUID; store_id > 0; 1 <= headcount <= maxHeadcount
// writeErr: ErrNotFound->404, ErrBadState->409, else 500; uses httpx.Created/OK/Fail
```

### POSITIONS FindByID (already loads JD text - just needs an HTTP surface)
```go
// SOURCE: backend/internal/positions/model.go:63-86
// SELECT ... COALESCE(responsibilities,''), COALESCE(qualifications,'') ... WHERE id=$1
// returns full Position (model.go:21-31 has Responsibilities + Qualifications fields)
```

### FRONTEND base-ui Select (function-child value→label) - mirror for enum selects too
```tsx
// SOURCE: frontend/components/requisitions/RequisitionDialog.tsx:95-115
<Select value={positionId} onValueChange={(v) => setPositionId(v ?? "")}>
  <SelectTrigger className="w-full">
    <SelectValue placeholder={t("positionPlaceholder")}>
      {(v: string | null) => {
        const p = (positions ?? []).find((p) => p.id === v)
        if (!p) return t("positionPlaceholder")
        return locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th
      }}
    </SelectValue>
  </SelectTrigger>
  <SelectContent>
    {(positions ?? []).map((p) => (
      <SelectItem key={p.id} value={p.id}>
        {locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th}
      </SelectItem>
    ))}
  </SelectContent>
</Select>
```

### FRONTEND raw textarea (no Textarea UI component exists)
```tsx
// SOURCE: frontend/components/resume/OfferPanel.tsx:147-153
<label className="block space-y-1.5">
  <span className="text-xs font-medium text-foreground">{t("terms")}</span>
  <textarea
    value={terms}
    onChange={(e) => setTerms(e.target.value)}
    rows={3}
    className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
    placeholder={t("termsPlaceholder")}
  />
</label>
```

### FRONTEND query hook (mirror for usePosition)
```ts
// SOURCE: frontend/lib/queries.ts:194-199 (usePositions)
export function usePositions() {
  return useQuery({
    queryKey: ["positions"],
    queryFn: () => api.get<Position[]>("/api/v1/positions").then((r) => r.data),
  })
}
```

---

## Data Model (migration 000040)

### New columns on `vacancies` (the requisition row)
All nullable / defaulted - `vacancies` is shared with the PeopleSoft sync `Upsert` (explicit column list) so additions must not break it.

| Column | Type | Notes |
|---|---|---|
| `responsibilities` | `TEXT` | nullable; prefilled from `positions.responsibilities` |
| `qualifications` | `TEXT` | nullable; prefilled from `positions.qualifications` |
| `benefits` | `TEXT` | nullable; prefilled from `positions.benefits` (new column below) |
| `other_details` | `TEXT` | nullable; "Other" free text |
| `employment_type` | `VARCHAR(20)` | nullable; one of `full_time \| part_time \| contract \| seasonal` |
| `salary_min` | `INTEGER` | nullable; THB/month |
| `salary_max` | `INTEGER` | nullable; THB/month |
| `priority` | `VARCHAR(20)` | `NOT NULL DEFAULT 'normal'`; one of `normal \| urgent` |
| `open_reason` | `VARCHAR(20)` | nullable; one of `new_headcount \| replacement` |

### New column on `positions` (the catalog - enables Benefits prefill + Phase 2 portal display)
| Column | Type | Notes |
|---|---|---|
| `benefits` | `TEXT` | nullable; mirrors existing `positions.responsibilities`/`positions.qualifications`. Populated via the same seed/script path as the other JD text (no positions UI editor exists). |

JSON field names (match Go json tags + frontend types): `responsibilities, qualifications, benefits, other_details, employment_type, salary_min, salary_max, priority, open_reason`.

Allowed-value constants (define in `model.go`):
- `EmploymentFullTime/PartTime/Contract/Seasonal`
- `PriorityNormal/Urgent`
- `ReasonNewHeadcount/Replacement`

---

## Files to Change

| File | Action | Justification |
|---|---|---|
| `backend/migrations/000040_requisition_jd_fields.up.sql` | CREATE | ALTER vacancies ADD 9 nullable/defaulted columns |
| `backend/migrations/000040_requisition_jd_fields.down.sql` | CREATE | DROP the 9 columns |
| `backend/internal/requisitions/model.go` | UPDATE | Struct fields + CreateInput/UpdateInput + value constants |
| `backend/internal/requisitions/repository.go` | UPDATE | reqColumns, scanRequisition, Create INSERT, Update SET |
| `backend/internal/requisitions/handler.go` | UPDATE | createReq/updateReq DTOs + validation |
| `backend/internal/requisitions/handler_test.go` | UPDATE | Validation test cases |
| `backend/internal/positions/model.go` | UPDATE | Add `Benefits` field to `Position` struct + `benefits` to `FindByID` (and other SELECTs) |
| `backend/internal/positions/handler.go` | UPDATE | Add `GET /api/v1/positions/:id` returning full detail incl. benefits |
| `backend/internal/positions/handler_test.go` | UPDATE | Test new detail route (if pattern exists) |
| `frontend/lib/types.ts` | UPDATE | Extend `Requisition`, `RequisitionInput`; add `PositionDetail` |
| `frontend/lib/queries.ts` | UPDATE | Add `usePosition(id)` hook |
| `frontend/components/requisitions/RequisitionDialog.tsx` | UPDATE | New fields + prefill effect + wider dialog |
| `frontend/components/requisitions/RequisitionTable.tsx` | UPDATE | (Optional) priority badge column |
| `frontend/messages/en.json` | UPDATE | New keys under `requisitions` |
| `frontend/messages/th.json` | UPDATE | New keys under `requisitions` |

## NOT Building (out of scope - Phase 1)
- Career portal surfacing of JD (that is Phase 2 - `requisition-detailed-fields-phase2-portal.plan.md`).
- Feeding qualifications into AI screening criteria (explicitly declined).
- Target start date field (declined by user).
- A positions catalog editor UI (positions JD - incl. the new `benefits` column - is seeded/synced; we only READ it for prefill and ADD the empty column).
- Rich-text editor (plain `<textarea>` per existing convention).
- Changing the existing position/store/headcount fields or the open→approve→close lifecycle.
- `salary`/`priority`/`employment_type`/`open_reason` on the `positions` table (requisition-level only). NOTE: `benefits` IS added to `positions` (decision: candidates should see benefits → it needs a position-level source).

---

## Step-by-Step Tasks

### Task 1: Migration 000040 (up + down)
- **ACTION**: Create `backend/migrations/000040_requisition_jd_fields.up.sql` and `.down.sql`.
- **IMPLEMENT** (up):
  ```sql
  ALTER TABLE vacancies
    ADD COLUMN responsibilities TEXT,
    ADD COLUMN qualifications    TEXT,
    ADD COLUMN benefits          TEXT,
    ADD COLUMN other_details     TEXT,
    ADD COLUMN employment_type   VARCHAR(20),
    ADD COLUMN salary_min        INTEGER,
    ADD COLUMN salary_max        INTEGER,
    ADD COLUMN priority          VARCHAR(20) NOT NULL DEFAULT 'normal',
    ADD COLUMN open_reason       VARCHAR(20);

  ALTER TABLE positions
    ADD COLUMN benefits TEXT;
  ```
  (down): `ALTER TABLE positions DROP COLUMN benefits;` then `ALTER TABLE vacancies DROP COLUMN ...` for all 9 (reverse order).
- **MIRROR**: `000029_requisitions.up.sql:16-22` (nullable/defaulted ALTER); `000010_position_jd_text.up.sql:4` (the `positions.qualifications TEXT` add - `positions.benefits` mirrors it exactly).
- **GOTCHA**: All `vacancies` columns nullable except `priority` (which is defaulted) → PeopleSoft `Upsert` and the `seed_vacancies*.sql` scripts keep working untouched. `positions.benefits` is plain nullable TEXT. Do NOT add CHECK constraints (status is free-form VARCHAR by convention; enforce values in app code).
- **VALIDATE**: `migrate up` then `migrate down` on a throwaway PG; `\d vacancies` and `\d positions` show/remove the columns cleanly.

### Task 2: Backend model - struct + inputs + constants
- **ACTION**: Extend `Requisition`, `CreateInput`, `UpdateInput` in `backend/internal/requisitions/model.go`; add value constants.
- **IMPLEMENT**:
  - `Requisition`: add `Responsibilities string`, `Qualifications string`, `Benefits string`, `OtherDetails string` (json `other_details`), `EmploymentType string`, `SalaryMin *int`, `SalaryMax *int`, `Priority string`, `OpenReason string`.
  - `CreateInput`: add `Responsibilities, Qualifications, Benefits, OtherDetails string`, `EmploymentType string`, `SalaryMin, SalaryMax *int`, `Priority string`, `OpenReason string`.
  - `UpdateInput`: add pointer variants `*string`/`*int` for each (sparse update).
  - Constants block: `EmploymentFullTime = "full_time"` … `PriorityNormal = "normal"`, `PriorityUrgent = "urgent"`, `ReasonNewHeadcount = "new_headcount"`, `ReasonReplacement = "replacement"`; plus exported slices/`map[string]bool` validators e.g. `validEmployment`, `validPriority`, `validReason`.
- **MIRROR**: existing `Requisition`/`CreateInput`/`UpdateInput` shape (model.go:38-90) and status consts (model.go:21-29).
- **GOTCHA**: Use `*int` for salary so "not provided" ≠ "0". Default `Priority` to `normal` when empty (set in handler).
- **VALIDATE**: `go build ./internal/requisitions/...`.

### Task 3: Repository - projection, scan, insert, update
- **ACTION**: Update `reqColumns`, `scanRequisition`, `Create`, `Update` in `repository.go`.
- **IMPLEMENT**:
  - `reqColumns`: append `, v.responsibilities, v.qualifications, v.benefits, v.other_details, v.employment_type, v.salary_min, v.salary_max, v.priority, v.open_reason` (use `COALESCE(...,'')` for the TEXT/VARCHAR ones to scan into `string`; leave `salary_min/max` raw to scan into `*int`).
  - `scanRequisition`: append the matching scan targets **in the same order** (`&r.Responsibilities, &r.Qualifications, &r.Benefits, &r.OtherDetails, &r.EmploymentType, &r.SalaryMin, &r.SalaryMax, &r.Priority, &r.OpenReason`).
  - `Create`: extend INSERT column list + `VALUES` placeholders + the `QueryRow(... )` arg list with the new CreateInput fields. Keep `priority` defaulted: pass `in.Priority` (handler guarantees non-empty).
  - `Update`: add a sparse `add(col, val)` block for each non-nil UpdateInput pointer (mirror existing).
- **MIRROR**: repository.go:18-26 / 106-151.
- **GOTCHA (CRITICAL)**: scan is **positional** - column order in `reqColumns` must exactly match the order of `&r.X` args in `scanRequisition`, and `List`/`getByID` both use the same projection so they stay correct automatically. Append-at-end on both sides to avoid reshuffling existing offsets.
- **VALIDATE**: `go build ./...`; a `getByID` round-trip in a repo test (or rely on handler test stub).

### Task 4: Handler - DTOs + validation
- **ACTION**: Extend `createReq`/`updateReq` and validation in `handler.go`.
- **IMPLEMENT**:
  - Add JSON fields to `createReq` (values) and `updateReq` (pointers).
  - Validation in `Create`: if `employment_type != ""` must be in `validEmployment` else 400; if `open_reason != ""` must be in `validReason`; `priority` → default to `PriorityNormal` when empty, else must be in `validPriority`; salary: if both set, `salary_min >= 0`, `salary_max >= salary_min` else 400; optional length cap on text fields (e.g. `<= 5000` chars) → 400 with clear message.
  - `Update`: validate the same for any provided pointer field.
- **MIRROR**: handler.go:99-116 validation style + `httpx.Fail` error messages; `writeErr` mapping (236-246).
- **GOTCHA**: Keep new fields OPTIONAL so existing create payloads still succeed (back-compat with current frontend during rollout). Trim whitespace; treat whitespace-only text as empty.
- **VALIDATE**: `go test ./internal/requisitions/...`.

### Task 5: Backend positions - Benefits field + detail endpoint (for prefill)
- **ACTION**: Add `Benefits` to the `Position` struct + SELECTs in `backend/internal/positions/model.go`, then add `GET /api/v1/positions/:id` in `handler.go`.
- **IMPLEMENT**:
  - `model.go`: add `Benefits string \`json:"benefits"\`` to the `Position` struct (after `Qualifications`); add `COALESCE(benefits,'')` to the column list in `FindByID` (and `FindByPSCode`/`ListAll` if they use the shared SELECT) and add `&p.Benefits` to the matching scan in the SAME positional order.
  - `handler.go`: new `Detail` handler: parse `:id` as UUID (400 on bad), call `repo.FindByID(ctx, id)` (now also loads benefits), map to a response incl. `id, title_th, title_en, level, responsibilities, qualifications, benefits`; `ErrNotFound`→404. Register route `app.Get("/api/v1/positions/:id", h.Detail)` next to the existing `GET /api/v1/positions`.
- **MIRROR**: model.go:63-86 (`FindByID` SELECT/scan); handler.go:35-51 (route registration + mapping); `public/handler.go:130-143` GetPosition shape (but DO include the JD text here - this is the authenticated dashboard endpoint).
- **GOTCHA**: positions SELECT/scan is positional like requisitions - append `benefits` at the END of the column list AND the scan args together. This is the authenticated dashboard route (no `/public` prefix) - it CAN return JD text. Confirm the positions route group has the same auth middleware as other dashboard routes (registered in `main.go` alongside authenticated routes).
- **VALIDATE**: `go test ./internal/positions/...`; manual `curl` with auth cookie returns responsibilities/qualifications/benefits.

### Task 6: Frontend types
- **ACTION**: Extend `frontend/lib/types.ts`.
- **IMPLEMENT**:
  - `Requisition`: add `responsibilities, qualifications, benefits, other_details, employment_type, priority, open_reason: string` and `salary_min, salary_max: number | null`.
  - `RequisitionInput`: add optional `responsibilities?, qualifications?, benefits?, other_details?, employment_type?, priority?, open_reason?: string`, `salary_min?, salary_max?: number`.
  - Add `PositionDetail { id: string; title_th: string; title_en: string; level: string; responsibilities: string; qualifications: string; benefits: string }`.
- **MIRROR**: types.ts:285-314, 277-281.
- **VALIDATE**: `pnpm tsc --noEmit` (or repo's typecheck).

### Task 7: Frontend usePosition hook
- **ACTION**: Add `usePosition(id, enabled)` to `frontend/lib/queries.ts`.
- **IMPLEMENT**:
  ```ts
  export function usePosition(id: string, enabled = true) {
    return useQuery({
      queryKey: ["position", id],
      queryFn: () => api.get<PositionDetail>(`/api/v1/positions/${id}`).then((r) => r.data),
      enabled: enabled && !!id,
      staleTime: 5 * 60 * 1000,
    })
  }
  ```
- **MIRROR**: queries.ts:194-199 (usePositions) + useStores staleTime (358-364).
- **VALIDATE**: typecheck.

### Task 8: RequisitionDialog - new fields + prefill + width
- **ACTION**: Add the new inputs to `frontend/components/requisitions/RequisitionDialog.tsx`.
- **IMPLEMENT**:
  - New `useState` per field: `responsibilities, qualifications, benefits, other, employmentType, salaryMin, salaryMax, priority(default "normal"), openReason` - hydrate from `requisition?.*` in edit mode (the page already forces a keyed remount so initial state re-reads).
  - Enum selects (employment type / priority / reason): mirror the position Select **function-child** pattern but resolve value→localized label via a static map of `t(...)` keys (so the trigger shows the Thai/English label, not the raw enum value).
  - Salary: two `<Input type="number" min={0}>` side by side; blank allowed.
  - Textareas (responsibilities, qualifications, benefits, other): mirror OfferPanel raw `<textarea>` (rows 3-5). Add a small helper hint under the JD group: an i18n line stating these describe the standard JD for the approval record (so HR understands edits here are internal, since the public portal shows the position catalog JD - see Summary).
  - Prefill: `const pos = usePosition(positionId, mode === "create" && positionId !== "")`. In a `useEffect` keyed on `pos.data` (create mode only), set responsibilities ← `pos.data.responsibilities`, qualifications ← `pos.data.qualifications`, benefits ← `pos.data.benefits`. **Re-prefill on position switch:** switching the position in create mode RE-fills all three JD textareas from the newly selected position (overwrites), so the JD always reflects the chosen position; this is predictable and simpler than tracking manual edits. Never prefill in edit mode (edit hydrates from the existing requisition).
  - `submit`: include all new fields in the `RequisitionInput`; convert salary via `salaryMin === "" ? undefined : Number(salaryMin)` (and max); send `priority` always.
  - Dialog: change `sm:max-w-md` → `sm:max-w-2xl`; wrap the form body in a scroll container (`max-h-[70vh] overflow-y-auto` on the fields wrapper) so it fits.
- **MIRROR**: RequisitionDialog.tsx:47-84, 95-146; OfferPanel.tsx:147-153.
- **GOTCHA**: `canSubmit` must stay based ONLY on the required trio (position, store, headcount≥1) - new fields are optional, do not gate submit on them. base-ui Select enum: use the function-child resolver or the trigger shows the raw enum string.
- **VALIDATE**: typecheck; manual: create dialog renders all fields, picking a position fills responsibilities/qualifications/benefits, switching position re-fills them, submit succeeds.

### Task 9: i18n keys (en + th)
- **ACTION**: Add keys under the `requisitions` namespace in both `frontend/messages/en.json` and `th.json` (block starts ~line 881).
- **IMPLEMENT** (parallel en/th): `fieldResponsibilities, fieldQualifications, fieldBenefits, fieldOther, fieldEmploymentType, fieldSalary, salaryMinPlaceholder, salaryMaxPlaceholder, salaryNegotiableHint, fieldPriority, fieldReason, jdInternalHint`, plus option labels `employment_full_time, employment_part_time, employment_contract, employment_seasonal, priority_normal, priority_urgent, reason_new_headcount, reason_replacement`, and placeholders for each textarea. (`jdInternalHint` TH example: `"รายละเอียด JD นี้ใช้สำหรับบันทึกการอนุมัติ ส่วนที่แสดงบนประกาศงานคือ JD กลางของตำแหน่ง"`.)
  - TH examples: `fieldResponsibilities: "หน้าที่ความรับผิดชอบ"`, `fieldQualifications: "คุณสมบัติผู้สมัคร"`, `fieldBenefits: "สวัสดิการ"`, `fieldOther: "รายละเอียดเพิ่มเติม"`, `fieldEmploymentType: "ประเภทการจ้าง"`, `fieldSalary: "ช่วงเงินเดือน"`, `fieldPriority: "ความเร่งด่วน"`, `fieldReason: "เหตุผลการเปิดรับ"`, `employment_full_time: "เต็มเวลา"`, `priority_urgent: "เร่งด่วน"`, `reason_replacement: "ทดแทนคนลาออก"` etc.
- **MIRROR**: existing requisitions keys (`fieldPosition`, `fieldStore`, `fieldHeadcount`, en/th lines ~906-921).
- **GOTCHA**: Keys MUST be added to BOTH files with identical key sets, or next-intl throws missing-key. Keep code identifiers English; labels Thai/English.
- **VALIDATE**: app builds; switch locale, all labels render (no raw key strings).

### Task 10 (optional): RequisitionTable priority badge
- **ACTION**: Add a small priority indicator to `RequisitionTable.tsx` (e.g. an "เร่งด่วน" badge when `priority === "urgent"`).
- **IMPLEMENT**: Mirror the status badge; gate on `priority === "urgent"`.
- **MIRROR**: RequisitionTable.tsx status badge + `status_${x}` label pattern.
- **VALIDATE**: typecheck; urgent rows show the badge.

---

## Testing Strategy

### Unit Tests (backend handler - extend handler_test.go)
| Test | Input | Expected | Edge? |
|---|---|---|---|
| Create with full JD | valid position/store/headcount + all new fields | 201, fields persisted (stub captures CreateInput) | no |
| Create back-compat | only position/store/headcount (no new fields) | 201 (new fields optional) | yes |
| Create bad employment_type | `employment_type:"weird"` | 400 | yes |
| Create salary inverted | `salary_min:30000, salary_max:10000` | 400 | yes |
| Create bad priority | `priority:"nope"` | 400 | yes |
| Create default priority | omit priority | 201, CreateInput.Priority == "normal" | yes |
| Update single JD field | `{responsibilities:"x"}` | 200, only that field in UpdateInput | yes |
| Positions detail | GET /positions/:id valid | 200 incl responsibilities/qualifications | no |
| Positions detail bad id | GET /positions/not-a-uuid | 400 | yes |

### Edge Cases Checklist
- [ ] Empty new fields (all optional) → create still 201
- [ ] Whitespace-only text → treated as empty
- [ ] salary_min set, salary_max blank (and vice versa) → accepted
- [ ] Text field at max length (5000) vs over → 400 over
- [ ] Prefill never overwrites user-typed text; never runs in edit mode
- [ ] PeopleSoft `Upsert` path unaffected (new columns nullable/defaulted)

---

## Validation Commands

### Static Analysis
```bash
cd backend && go build ./... && go vet ./internal/requisitions/... ./internal/positions/... && gofmt -l internal/requisitions internal/positions
```
EXPECT: builds, vet clean, no unformatted files (of the ones we touched)

```bash
cd frontend && pnpm tsc --noEmit   # or the repo's typecheck script
```
EXPECT: zero type errors

### Unit Tests
```bash
cd backend && go test -race ./internal/requisitions/... ./internal/positions/...
```
EXPECT: all pass

### Full Test Suite
```bash
cd backend && go test ./...
```
EXPECT: 0 FAIL (no regressions)

### Database Validation
```bash
# against throwaway PG
migrate -path backend/migrations -database "$PG_URL" up
psql "$PG_URL" -c '\d vacancies'   # 9 new columns present
migrate -path backend/migrations -database "$PG_URL" down 1
psql "$PG_URL" -c '\d vacancies'   # columns gone
```
EXPECT: clean up + down

### Manual Validation
- [ ] Dashboard `/requisitions` → New requisition → pick a position → responsibilities + qualifications auto-fill
- [ ] Edit existing requisition → fields load, no auto-prefill clobber
- [ ] Fill salary/employment/priority/reason → Create → row persists; reopen edit shows saved values
- [ ] Existing approve/close flow still works on the new rows

---

## Acceptance Criteria
- [ ] HR can capture Responsibilities, Qualifications, Benefits, Other on a requisition
- [ ] Employment type, Salary range, Priority, Reason captured
- [ ] Responsibilities + Qualifications + Benefits prefill from the linked position (create mode; re-prefill on position switch)
- [ ] `positions.benefits` column added (seed-populated; no UI editor)
- [ ] Form shows the "JD is internal / portal uses position JD" hint
- [ ] Position/Store/Headcount unchanged and still work
- [ ] All new fields optional; old create payloads still succeed
- [ ] All validation commands pass

## Completion Checklist
- [ ] Positional scan order verified (reqColumns ↔ scanRequisition)
- [ ] New columns nullable/defaulted (PS sync safe)
- [ ] Validation mirrors existing handler style + writeErr mapping
- [ ] i18n keys in BOTH en.json and th.json
- [ ] Tests follow handler_test.go stub pattern
- [ ] No hardcoded labels in components (all via next-intl)

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Positional scan misalignment → wrong field values | Medium | High | Append-at-end on both reqColumns and scanRequisition; round-trip test |
| New NOT NULL without default breaks PS `Upsert`/seed scripts | Low | High | Only `priority` is NOT NULL and it has a DEFAULT; rest nullable |
| Dialog too tall with 4 textareas | Medium | Low | `sm:max-w-2xl` + scrollable body |
| Positions detail route missing auth | Low | Medium | Register in the authenticated route group; verify middleware |

## Notes
- Requisitions are `vacancies` rows with `source='manual'`; there is no separate table. Lifecycle stays `pending_approval → open → closed` (cancelled declared but unwired - out of scope).
- `headcount` already exists end-to-end (1-999) - item "เลือกจำนวนที่ต้องการเปิดรับ" is already satisfied; no change.
- **Known limitation - post-approval immutability:** `Update` is guarded to `status='pending_approval'`, so once a requisition is approved (`status='open'`) its JD/fields can no longer be edited (no reopen path exists). Pre-existing behavior, but higher impact now that rich JD lives on the row. Flag to the user; do NOT add a reopen path in this phase unless requested.
- **Benefits source of truth:** `positions.benefits` is the canonical, candidate-facing benefits text (Phase 2 portal). It has no UI editor - populate it via the same seed/script path as the existing `positions.responsibilities`/`qualifications` (e.g. `scripts/seed_vacancies_master_jd.sql` or a sibling). The requisition's `benefits` is a prefilled internal copy.
- **HR edits do not publish:** because the portal is position-keyed (Phase 2), candidate-facing JD = position catalog text, NOT the per-requisition edits. The `jdInternalHint` label (Task 9) communicates this in the form.
- Phase 2 (`requisition-detailed-fields-phase2-portal.plan.md`) surfaces position-level responsibilities/qualifications/benefits on the career portal detail page; requisition-level-only fields (other/salary/priority/reason/store) stay internal per the agreed scope.
- **Verify the frontend typecheck command before running validation** - confirm the dashboard's package manager + script (the repo may use `pnpm`, `yarn`, or `npm`; the hooks reference `pnpm tsc` but confirm against `package.json`).
- HARD RULES for implementation: no em dash in any output; deploy after dev (api + dashboard + migration 000040); NO Co-Authored-By in commits.

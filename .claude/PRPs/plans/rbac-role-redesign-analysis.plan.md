# RBAC Role & Permission Redesign — Analysis & Design

**Status:** Analysis (pre-development) · **Date:** 2026-06-24
**Source:** Client-updated role/permission model + current-code gap analysis
**Schema:** current v41 → next migration `000042`

> เอกสารวิเคราะห์ก่อนพัฒนา ทุกข้อ requirement และการตัดสินใจถูก confirm กับลูกค้าแล้ว
> (บันทึกการตัดสินใจอยู่ท้ายเอกสาร) ยังเหลือ residual questions เล็กน้อยที่ flag ไว้

---

## 1. เป้าหมาย

เปลี่ยนโมเดล RBAC จาก **มิติเดียว** (geographic scope: all/subregion/store) เป็น **2 แกน**:

- **แกน A — Visibility scope:** `store` → `area` → `requisition` (ตำแหน่งที่เปิดเอง) → `all`
- **แกน B — Capability:** `read-only` vs `operate` vs `admin`

พร้อมเพิ่มกลไกใหม่: **area แบบ dynamic**, **candidate pool กลาง**, **lock ระหว่าง process**, และ **timeout 3 วันปล่อยลง pool**

---

## 2. โมเดล Role ใหม่ (แทนที่ role เดิมทั้งหมด — มี 7 role)

| role key | Visibility scope | Capability | operated-for-by | notes |
|----------|------------------|------------|-----------------|-------|
| `hr_store` | สาขาตัวเอง **+ central pool ทั้งหมด** | operate | (ตัวเอง) | operate ให้ sgm |
| `area_hr` | area ตัวเอง (dynamic, 10-20 สาขา) **+ central pool** | operate (เต็มภายใน area) | (ตัวเอง) | visibility = area only |
| `hiring_manager_store` (sgm) | เฉพาะ requisition ที่เปิดเอง | read-only (ops) **+ approve** | hr_store | ดู candidate อย่างเดียว แต่**เป็นผู้อนุมัติจ้าง** + รับ notification |
| `hiring_manager_ho` | เฉพาะ requisition ที่เปิดเอง | read-only (ops) **+ approve** | ta | ดู candidate อย่างเดียว แต่**เป็นผู้อนุมัติจ้าง** + รับ notification |
| `ta` | all | operate | (ตัวเอง) | head office recruiter |
| `auditor` | all | **read-only ล้วน** | — | ผู้ตรวจสอบ/PDPA ดูทั้งหมด ไม่แก้ |
| `super_admin` | all | admin | — | bypass |

**การตัดสินใจที่ยืนยันแล้ว:**
- area_hr เห็น+operate เฉพาะ area ตัวเอง (ไม่ใช่ทั้งหมด)
- TA = 1 role, operate ได้ทั้งหมด (ไม่มี viewer-only TA แยก)
- central pool: `hr_store` ทุกสาขา + `area_hr` + `ta` เห็นและดึงไป operate ได้ (แย่งกัน → ต้อง lock)
- **HM (sgm + ho): "read-only" หมายถึงไม่ operate candidate (ไม่แก้สถานะ/ไม่จัดสาขา/ไม่กรอกข้อมูล) แต่ HM = ผู้ตัดสินอนุมัติจ้าง (`approval.decide`)** — operate ≠ approve. ดู §7
- `auditor` = role ใหม่ read-only-all (แทน auditor เดิมที่เป็น read-only ทั้งหมด)

---

## 3. Gap Analysis (มีอยู่แล้ว vs สร้างใหม่)

| ต้องการ | ปัจจุบัน | งานที่ต้องทำ |
|---------|----------|--------------|
| scope=store | ✅ `applications.assigned_store_id` + `scope.go:ApplicationsClause` | reuse |
| scope=area (dynamic) | ❌ มีแค่ `subregion` hardcode 13 ค่าใน `stores/subregion.go` แก้ผ่าน UI ไม่ได้ | **สร้าง `areas`/`area_stores`/`user_areas` + admin UI + scope kind `area`** |
| scope=requisition (HM เปิดเอง) | ❌ `vacancies.created_by` เป็น UUID audit (ไม่ใช่ FK, SSO อาจไม่มี row) + **applications ไม่มี `vacancy_id`** | **เพิ่ม `vacancies.hiring_manager_user_id` (FK) + `applications.vacancy_id` + scope kind `requisition`** |
| read-only role | ⚠️ แยกได้ผ่าน permission (ไม่ให้ write) | นิยาม role read-only + scope requisition |
| central pool | ✅ ฐานมี: `applications.talent_pool=true` + `assigned_store_id=NULL` + `candidate_accounts` | นิยาม visibility rule + การดึงออกจาก pool |
| lock candidate | ❌ ไม่มี (`reviewed_by` = audit เท่านั้น) | **เพิ่มกลไก lock ระดับ candidate + TTL** |
| timeout 3 วัน → pool | ❌ ไม่มี (มี pattern `approval:sla_sweep` ให้ลอก) | **สร้าง release sweep + timestamp** |
| เปลี่ยน area/ผู้ดูแลบ่อย | ❌ subregion มาจาก token claim ตายตัว | เก็บ area assignment ใน DB แก้ผ่าน admin |

✅ **dynamic RBAC ปัจจุบันรองรับ `scope_kind` ต่อ role อยู่แล้ว** (CHECK `all/subregion/store`) → ขยายเพิ่ม `area`,`requisition` ได้

---

## 4. Data Model Changes (DDL sketch — migration 000042)

```sql
-- 4.1 Area (dynamic, admin-managed)
CREATE TABLE areas (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        VARCHAR(120) NOT NULL,
  active      BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE area_stores (              -- M:N, แก้ได้บ่อย
  area_id   UUID NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
  store_no  INTEGER NOT NULL REFERENCES stores(store_no),
  PRIMARY KEY (area_id, store_no)
);
CREATE TABLE user_areas (               -- ใครดูแล area ไหน (เปลี่ยนบ่อย)
  user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  area_id  UUID NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, area_id)
);

-- 4.2 Requisition ownership (hiring manager จริง)
ALTER TABLE vacancies ADD COLUMN hiring_manager_user_id UUID REFERENCES users(id);
-- resolve จาก email ตอนเปิด req (เหมือน pattern hrauth.FindByEmail ที่ใช้แก้ created_by interview)

-- 4.3 Link application ↔ vacancy (ปัจจุบันขาด)
ALTER TABLE applications ADD COLUMN vacancy_id UUID REFERENCES vacancies(id);
-- set ตอน branch assign (assigner หา open vacancy อยู่แล้ว) + ตอน manual assignment

-- 4.4 Candidate lock (ระดับ candidate / central identity)
CREATE TABLE candidate_locks (
  account_id   UUID PRIMARY KEY REFERENCES candidate_accounts(id) ON DELETE CASCADE,
  locked_by    UUID NOT NULL REFERENCES users(id),
  locked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL          -- หมดอายุเอง
);
-- หมายเหตุ: candidate ที่ไม่มี account_id → ใช้ canonical candidates.id เป็น key สำรอง (ดู residual Q)

-- 4.5 3-day pickup tracking
ALTER TABLE applications ADD COLUMN picked_up_at  TIMESTAMPTZ;   -- action แรกของ HR สาขา
ALTER TABLE applications ADD COLUMN picked_up_by  UUID REFERENCES users(id);
ALTER TABLE applications ADD COLUMN released_to_pool_at TIMESTAMPTZ;  -- audit ตอน sweep ปล่อยลง pool

-- 4.6 ขยาย scope kinds
ALTER TABLE rbac_roles DROP CONSTRAINT <scope_kind_check>;
ALTER TABLE rbac_roles ADD CONSTRAINT scope_kind_check
  CHECK (scope_kind IN ('all','subregion','store','area','requisition'));
-- + reseed roles ใหม่ + role_permissions
```

อินเด็กซ์ที่ต้องมี: `area_stores(store_no)`, `user_areas(user_id)`, `vacancies(hiring_manager_user_id)`, `applications(vacancy_id)`, `applications(assigned_store_id, picked_up_at) WHERE picked_up_at IS NULL` (สำหรับ sweep), `candidate_locks(expires_at)`

---

## 5. Scope Resolution Logic (scope.go ใหม่)

ขยาย `internal/rbac/scope.go` — `Scope` struct เพิ่ม `UserID`, `AreaIDs []` (resolve ตอน build). Clause ใหม่ต่อ kind:

```text
all:          (ว่าง)
store:        assigned_store_id = $store
              OR (talent_pool = true AND assigned_store_id IS NULL)   -- central pool (ทุก hr_store เห็น)
area:         assigned_store_id IN (SELECT store_no FROM area_stores
                 WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $me))
              OR (talent_pool = true AND assigned_store_id IS NULL)   -- + central pool
requisition:  vacancy_id IN (SELECT id FROM vacancies WHERE hiring_manager_user_id = $me)
              -- HM เห็นเฉพาะคนในตำแหน่งที่ตัวเองเปิด, ไม่เห็น pool, ไม่เห็นอื่น (default-deny)
```

**สำคัญ:** subregion ถูก**ถอดออกจาก RBAC scope** (แทนด้วย area) แต่ `subregion` ยัง**คงอยู่ในฐานะ attribute สำหรับ branch assigner** (`vacancies` matching ยังใช้ subregion หา store) — แยกบทบาทกันชัดเจน

CandidatesClause / AccountsClause / VacanciesClause ต้องอัปเดต logic เดียวกัน (area + requisition + pool)

---

## 6. Candidate State & Lifecycle

**2 ประเภท candidate:**
- **store-specific:** `assigned_store_id` set, `talent_pool=false` — สาขานั้นรับผิดชอบ
- **central pool:** `assigned_store_id=NULL`, `talent_pool=true` — ใครก็ดึงได้ (hr_store/area_hr/ta)

**Lifecycle + timer + lock:**
```
ยื่นสมัคร (created_at)
  → pipeline assign สาขา (assigned_store_id) → store-specific, timer เริ่ม
  → [HR สาขา action ใดๆ] → picked_up_at set → timer หยุด → process ต่อ
  → [ไม่มี action ใน 3 วัน] → sweep: assigned_store_id=NULL, talent_pool=true,
        released_to_pool_at set → เข้า central pool → แจ้งเตือน
  → ใน pool: ใครจะ operate ต้อง acquire lock ก่อน (กันชนกัน)
```

### 6.1 Lock (ระดับ candidate)
- key = `account_id` (central identity) → ครอบทุก application ของคนนั้น
- คนอื่น**ยังเห็นได้** แต่ **operate ไม่ได้** ขณะถูก lock (แสดง "กำลังถูกดำเนินการโดย X")
- **หมดอายุเอง** (เสนอ default 30 นาที — ดู residual Q) + ปลดเองได้ + admin override
- acquire = `INSERT ... ON CONFLICT` (atomic), เช็ค `expires_at` ก่อน

### 6.2 Release sweep (3 วัน)
- ลอก pattern `cmd/scheduler` → `approval:sla_sweep`
- cron task ใหม่ `pool:release_sweep` (gated env flag) → worker handler
- เงื่อนไข: `assigned_store_id IS NOT NULL AND picked_up_at IS NULL AND talent_pool=false AND created_at < now() - interval '3 days'`
- action: ย้ายลง central pool + notify (HR สาขาเดิม + อาจ area_hr/ta)
- **นับจาก created_at (ยื่นสมัคร)**, calendar days (ยืนยันแล้ว: นับจากยื่นสมัคร + action ใดๆ = รับ)

---

## 7. Hiring Manager: read-only ops + approver + Notification

**นิยาม "read-only" ของ HM = ไม่ operate candidate แต่เป็นผู้อนุมัติจ้าง** (operate ≠ approve):
- permission set: `candidates.view` + `applications.view` + **`approval.decide`** (อนุมัติ/ปฏิเสธจ้างบน candidate ใน req ตัวเอง) — **ไม่มี** operate/write perms (แก้สถานะ pipeline, จัดสาขา, กรอกข้อมูล, lock ฯลฯ)
- scope = `requisition` — เห็น+อนุมัติเฉพาะ candidate ในตำแหน่งที่ตัวเองเปิด **ไม่จำกัด store** (ยืนยันแล้ว)
- **HM เป็นผู้อนุมัติแทน sgm เดิมใน approval chain** (ยืนยันแล้ว: "hiring manager เป็นผู้อนุมัติ") → ดู §8 การ remap
- **Notification:** มีผู้สมัครใหม่/อัปเดตในตำแหน่งที่ HM เปิด → แจ้ง HM (email/Teams) ไปดู+อนุมัติ; ผู้ operate (hr_store สำหรับ sgm, ta สำหรับ ho) เป็นคนลงมือ process
- "เปิดตำแหน่ง" = สร้าง/เป็นเจ้าของ requisition (`vacancies.hiring_manager_user_id = HM`)

> ⚠️ ยืนยันการตีความ: คำตอบ "HM ดูอย่างเดียว" (รอบ 2) + "HM เป็นผู้อนุมัติ" (residual) ถูก reconcile เป็น
> "read-only บน operations + เป็น approver" — รอ confirm จากลูกค้าอีกครั้งก่อน implement

---

## 8. Migration ของ user เดิม → role ใหม่ (proposed, ต้อง confirm)

| role เดิม | scope เดิม | → role ใหม่ (ยืนยันแล้ว) | หมายเหตุ |
|-----------|-----------|---------------------|----------|
| `hr_staff` | store | `hr_store` | ตรง |
| `hr_manager` | store | **`area_hr`** | ยืนยันแล้ว — ต้องสร้าง area + assign user_areas |
| `sgm` | store + approver | `hiring_manager_store` | read-only ops **แต่ยังเป็น approver** (operate งานยกให้ hr_store) |
| `operation_director` | subregion | `area_hr` | subregion≠area → ต้องสร้าง area mapping |
| `regional_director` | all | `ta` | |
| `auditor` | all (read) | **`auditor` (ใหม่)** | ยืนยันแล้ว — role read-only-all |
| `super_admin` | all | `super_admin` | คงเดิม |

⚠️ approval workflow (000022: levels อ้าง `hr_staff|hr_manager|sgm|regional_director`) ต้อง remap:
**ผู้อนุมัติที่เคยเป็น sgm → เปลี่ยนเป็น hiring manager (HM)** ในระดับที่เกี่ยวข้อง (ยืนยันแล้ว) +
ทบทวน level อื่นที่อ้าง role เดิม (hr_manager→area_hr, regional_director→ta)

---

## 9. ไฟล์/แพ็กเกจที่กระทบ

**Backend:**
- `internal/rbac/{scope.go,legacy.go,permissions.go,authorizer.go}` — scope kinds ใหม่, role/permission ใหม่
- `migrations/000042_*` — ตาราง area, vacancy HM, application vacancy_id+lock+timer, reseed RBAC
- `internal/areas/` (ใหม่) — CRUD area + area_stores + user_areas + admin API
- `internal/applications/{repository.go,handler.go,model.go}` — vacancy_id, lock acquire/release, scope ใหม่, picked_up stamping
- `internal/branch/assigner.go` — capture vacancy_id ตอน assign
- `internal/requisitions/{model.go,handler.go}` — hiring_manager_user_id (resolve email)
- `internal/candidates/`, `internal/profiles/` — scope ใหม่ + lock state ใน response
- `internal/pipeline/process.go` — set vacancy_id, timer start
- `cmd/scheduler/main.go` + worker handler — `pool:release_sweep`
- `internal/notify/` — HM notification + release notification
- `internal/hrauth/` — reuse FindByEmail สำหรับ resolve HM
- approval workflow (000022 consumers) — remap roles, เอา sgm ออก

**Frontend (dashboard):**
- admin: หน้าจัดการ area (สร้าง/แก้, ย้ายสาขา, assign area_hr)
- admin: หน้า assign role/scope ต่อ user (เลือก area/store)
- candidate list/detail: lock state UI ("กำลังถูกดำเนินการ"), ปุ่ม claim/release
- read-only mode สำหรับ HM (ซ่อนปุ่ม operate)
- requisition: ช่อง hiring manager
- nav/permission gating ตาม role ใหม่

---

## 10. Residual Questions — RESOLVED (2026-06-24)

1. **Lock TTL = 30 นาที** ต่ออายุอัตโนมัติเมื่อมี activity (ตัดสินโดยทีม dev, default)
2. **`auditor` → สร้าง role read-only-all ใหม่** (ยืนยันแล้ว) — role set จึงมี 7 ตัว
3. **`hr_manager` → `area_hr`** (ยืนยันแล้ว)
4. **lock key = canonical `candidates.id`** (ไม่ใช่ account_id — ครอบคลุม bulk/PS/legacy ที่ไม่มี account) (ตัดสินโดยทีม dev)
5. **Approval: hiring manager เป็นผู้อนุมัติแทน sgm** (ยืนยันแล้ว) — HM read-only ops แต่ถือ `approval.decide` ดู §7
6. **HM เห็นทุก candidate ใน req ไม่จำกัด store** (ยืนยันแล้ว) — scope = requisition ล้วน
7. **area ซ้อนกันได้ (M:N)** — 1 สาขาอยู่ได้หลาย area, 1 area_hr ดูแลได้หลาย area (ตัดสินโดยทีม dev, schema รองรับแล้ว)

### เหลือ confirm ก่อน implement
- ⚠️ **การตีความ HM "read-only + approver"** (§7) — operate≠approve ถูกต้องไหม?
- approval chain ใหม่: HM อยู่ level ไหน, มีกี่ level, ใครอยู่ level อื่น (hr_manager→area_hr, regional_director→ta) — ต้อง map รายละเอียด

---

## 11. Effort & Phasing (ประมาณการ)

แนะนำแบ่ง 3 sub-phase:
- **P1 — Role/scope core:** migration 000042 (area tables, scope kinds), scope.go ใหม่, reseed roles, user migration, area admin API+UI · ~20-25 md
- **P2 — Pool + lock + timeout:** vacancy_id link, candidate lock, release sweep, requisition scope, HM notification · ~18-22 md
- **P3 — Frontend + read-only + approval remap:** read-only HM UI, lock UI, role-assign UI, approval workflow remap, UAT · ~15-20 md

**รวม ~55-65 md** (ยังไม่รวมการ migrate ข้อมูล prod จริง + UAT รอบใหญ่)

---

## บันทึกการตัดสินใจ (confirmed กับลูกค้า 2026-06-24)

รอบ 1: area_hr=area-only / TA=1role operate-all / pool=hr_store ทุกสาขา+area_hr+ta / replace role เดิม
รอบ 2: lock=ระดับ candidate+หมดอายุเอง / timer=นับจากยื่นสมัคร, action ใดๆ=รับ / HM=read-only ops+notification / area=จัดการใน admin UI
รอบ 3 (residual): auditor=สร้าง read-only-all / hr_manager→area_hr / approver=hiring manager / HM เห็นข้าม store ตาม req
ตัดสินโดย dev: lock TTL=30นาที / lock key=canonical candidates.id / area=M:N
RECONCILE: HM "read-only" (รอบ2) + "เป็นผู้อนุมัติ" (รอบ3) = read-only ops + approver (operate≠approve) — รอ confirm

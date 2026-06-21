# Records of Processing Activities (ROPA)

**Thailand Personal Data Protection Act B.E. 2562 (PDPA), Section 39**

This document records the processing activities of the AI HR Recruitment &
Screening System ("the System"). It is the controller's register of how personal
data is collected, used, disclosed, retained, and protected. Review and update it
whenever a processing activity, processor, retention rule, or data category
changes.

| Field | Value |
|---|---|
| Data controller | CP Axtra Public Company Limited |
| System | AI HR Recruitment & Screening System |
| Document owner | Data Protection Officer (DPO) |
| Last reviewed | 2026-06-21 |
| Status | Draft - confirm DPO details and Azure regions with the operator before filing |

> Values marked **[confirm]** are deployment- or policy-dependent and must be
> verified by the operator / DPO before this register is treated as authoritative.

---

## 1. Controller and DPO

- **Controller:** CP Axtra Public Company Limited (Makro / Lotus's).
- **DPO contact:** **[confirm]** name / email / phone. Surfaced to data subjects on
  the `/privacy` page (Phase 4) and configured via `PDPA_DPO_NAME` / `PDPA_DPO_EMAIL`
  / `PDPA_DPO_PHONE` (Phase 5 DPO console - pending).
- **Basis for DPO appointment (s.41(2)):** the System processes **special-category
  data** at scale (Thai national ID, plus onboarding scans including health-check,
  house registration, and bank book), which requires a DPO.

---

## 2. Categories of data subjects

- Job applicants / candidates (career-portal members and walk-in / referred
  candidates).
- Hired candidates transitioning to employees (onboarding documents).
- HR staff and approvers are **users of** the System; their account data is
  employment data on a separate basis and is **out of scope** of the candidate ROPA.

---

## 3. Categories of personal data

| Category | Examples | Special category (s.26)? |
|---|---|---|
| Identity | full name, national ID (`id_card`), date of birth | **Yes** - national ID |
| Contact | phone, email, address, province | No |
| Account / auth | portal email, LINE OAuth subject, Google OAuth subject, hashed sessions/OTPs | No |
| Application | resume file + parsed profile, OCR text, source channel | Possibly (free-text resumes may contain special-category data the subject volunteers) |
| AI assessment | AI score + breakdown, AI summary, red flags, fit analysis, pre-interview transcript | Derived |
| Interview | panel feedback (ratings, free-text), scheduling (Teams link, calendar id) | No |
| Offer / onboarding | salary, start date, offer terms, onboarding scans (`id_card`, `house_registration`, `education_certificate`, `bank_book`, `tax_document`, `photo`, `health_check`, `military_certificate`, `name_change`) | **Yes** - health-check + financial (bank book) |
| Consent ledger | consent given/withdrawn, version, source, IP | No |
| Audit | activity log (action, entity, actor, IP, user-agent) | No |

---

## 4. Processing activities

Each row is one processing activity with its purpose, lawful basis, data, retention,
and recipients.

| # | Activity | Purpose | Lawful basis | Data categories | Retention |
|---|---|---|---|---|---|
| P1 | Application intake | Receive and store applications (portal, walk-in, intake webhook) | Consent (s.19) + legitimate interest (recruitment) | Identity, contact, application | 365 days from creation; hired excluded |
| P2 | AI CV screening | OCR + parse + score resumes against the position JD | Consent + legitimate interest | Application, AI assessment | With the application (P1 window) |
| P3 | AI pre-interview | Conversational screening of invited candidates | Consent | Pre-interview transcript, AI assessment | With the application |
| P4 | Human interview | Scheduling + structured panel feedback | Legitimate interest | Interview scheduling + feedback | With the application |
| P5 | Approval / offer / onboarding | Multi-level approval, offer letters, onboarding document collection | Contract (pre-contractual) + legal obligation (employment records once hired) | Offer (salary/terms), onboarding scans (special category) | Hired: retained per HR/employment policy; not hired: P1 window |
| P6 | Talent pool re-engagement | Notify prior candidates of new openings | Consent | Identity, contact, outreach logs | P1 window |
| P7 | Notifications | LINE / email / Teams updates to candidates + HR | Consent (candidate) / legitimate interest (HR) | Contact, status snapshots | Transient; payload erased with the subject |
| P8 | Portal accounts | Candidate self-service (signup, profile, saved resume, status) | Consent / contract | Account, auth, application | Until erased; sessions/OTPs are short-lived |
| P9 | Reporting / executive overview | Aggregate recruitment funnel + workforce metrics | Legitimate interest | De-identified aggregates | Derived from P1-P5 |

---

## 5. Recipients and processors (incl. cross-border, s.28-29)

The System runs on Microsoft Azure (resource group `hrats-prod-rg`) and integrates
the processors below. Each is enabled by a `*_PROVIDER` config flag; when set to
`mock` no personal data leaves the System.

| Processor | Role | Personal data shared | Region / cross-border | Flag |
|---|---|---|---|---|
| Azure OpenAI (gpt-4o-mini) | CV scoring + pre-interview + fit analysis | Resume text, JD, chat | **[confirm]** `eastus` (US) - **cross-border** | `AI_PROVIDER=azure` |
| Azure AI Document Intelligence | Resume OCR | Resume file | **[confirm]** region per `AZURE_DOC_INTEL_ENDPOINT` | `AI_PROVIDER=azure` |
| Azure AI Search | Candidate search index | Name, province, status, score | **[confirm]** SE Asia | `AI_SEARCH_PROVIDER=azure` |
| Azure Blob Storage | Resume + onboarding + letter files | Resume, onboarding scans (special category) | **[confirm]** SE Asia | (storage) |
| Azure Communication Services | Candidate + HR email | Name, email, status | **[confirm]** region per `ACS_EMAIL_ENDPOINT` | `EMAIL_PROVIDER=real` |
| LINE | Login + candidate notifications | LINE OAuth subject, push messages | Japan / global - **cross-border** | `LINE_PROVIDER` / `NOTIFY_PROVIDER` |
| Google | OAuth login | Google OAuth subject, email | US / global - **cross-border** | `GOOGLE_PROVIDER=real` |
| Microsoft Graph / Teams | Interview calendar + meeting | Candidate name, interview time | **[confirm]** - `mock` on prod | `GRAPH_PROVIDER=real` |
| PeopleSoft | HRIS vacancy / headcount sync | Position + headcount (not candidate PII) | On-prem / **[confirm]** - `mock` on prod | `PS_PROVIDER=real` |

**Cross-border action (s.28):** Azure OpenAI (`eastus`), LINE, and Google place
personal data outside Thailand. Confirm an adequate safeguard (Microsoft/Google/LINE
DPAs + standard contractual clauses) is on file, and prefer Thailand / SE-Asia
regions for Azure resources where the model/service is available. **[confirm]**

---

## 6. Retention

- Default candidate retention: **365 days** from creation (`RETENTION_DAYS`, env-configurable).
- A scheduled sweep erases expired candidates: de-identifies PII, deletes
  PII-bearing rows + blobs, and removes the subject from the search index
  (PDPA Phase 1 `EraseSubject`).
- **Hired candidates are excluded** from the sweep - their data becomes employment
  data retained under HR policy / legal obligation.
- Sessions and OTPs are short-lived and purged on their own schedule.

---

## 7. Data subject rights (how the System fulfils them)

| Right (PDPA) | Implementation |
|---|---|
| Consent, withdrawable (s.19) | Versioned consent registry + withdrawal endpoint (Phase 2) |
| Access (s.30) + portability (s.31) | Portal self-service JSON export, `GET /me/export` (Phase 3) |
| Rectification (s.36) | Portal profile edit |
| Erasure (s.33) | Portal self-service erasure, `POST /me/erase`; hired/legal-hold requests queued for HR (Phase 3) |
| Object / restrict (s.32, 34) | Withdrawal + erasure cover the consent-based processing |
| Transparency | `/privacy` notice page, version-stamped (Phase 4) |

---

## 8. Security measures (s.37(1))

- Hashed sessions + OTPs (never plaintext); RBAC with per-role scope; role-gated
  admin endpoints.
- HTTPS / Azure Container Apps ingress; secrets in Azure secret store, not source.
- Rate limiting on public + login endpoints; real-client-IP handling behind the
  trusted proxy.
- Audit log of PDPA-relevant actions (actor + IP + user-agent). **Coverage hardening
  is Phase 5 Task 5.1 - pending.**

---

## 9. Breach handling (s.37(4))

- Obligation: notify the PDPC within **72 hours** of becoming aware of a breach, and
  affected subjects without undue delay when the risk is high.
- A breach register with a 72-hour countdown and PDPC notification content is
  **Phase 5 Task 5.3 - pending** (`data_breaches` table + admin console).

---

## 10. Open items before filing

- [ ] Confirm DPO name / email / phone and wire `PDPA_DPO_*`.
- [ ] Confirm the exact Azure region of each resource; record cross-border safeguards.
- [ ] Complete audit-coverage hardening (Task 5.1).
- [ ] Stand up the breach register (Task 5.3) and the PDPA admin console (Task 5.4).
- [ ] Legal review + DPO sign-off.

# UAT CV Test-Set

How to assemble the labeled CV set the parse-accuracy harness grades against.

> ⚠️ Real CVs are PII. Place them in `backend/e2e/cvset/` which is **gitignored**.
> Use them only on a staging stack. Never commit, never load into prod `hr_db`.

## Size & variety
Assemble **20–30** real CVs spread across this matrix (don't put them all in one bucket):

| Dimension | Variants |
|---|---|
| Language | Thai-only, English-only, mixed TH/EN |
| File type | PDF (digital), PDF (scanned image), DOCX, JPG/PNG photo |
| Layout | single-column, two-column, table-heavy, photo + sidebar |
| Length | 1-page, 2–3 page |
| Quality | clean, low-res scan, rotated/skewed |
| Content | fresh grad, experienced, career-changer, missing email/phone |

## File naming
For each CV, two files sharing a base name in `backend/e2e/cvset/`:
```
candidate-01.pdf            # or .docx / .png / .jpg
candidate-01.expected.json  # ground truth (you fill this by reading the CV)
```

## Ground-truth schema (`*.expected.json`)
Only include fields you want graded — omit a field to skip it (it won't be penalized).
```json
{
  "name": "สมชาย ใจดี",
  "phone": "081-234-5678",
  "email": "somchai@example.com",
  "education_level": "ตรี",
  "total_experience_months": 36,
  "skills": ["cashier", "POS", "inventory"],
  "languages": ["Thai", "English"]
}
```
Grading rules (see `backend/e2e/scorecard.go`):
- **name** — normalized exact match (case/space-insensitive).
- **phone** — digits-only exact match (formatting ignored).
- **email** — case-insensitive exact match.
- **education_level** — keyword contained in any parsed degree (TH or EN, either direction). Use a short token: `ตรี`/`bachelor`, `โท`/`master`, `ปวส`/`diploma`, etc.
- **total_experience_months** — correct if within **±12 months** of the parsed sum.
- **skills** — recall: fraction of expected skills found in the parsed skills (substring, normalized).
- **languages** — recall over expected languages.

## Tips for labeling
- `total_experience_months` = sum of each job's duration in months (estimate is fine; ±1yr tolerated).
- For `education_level`, pick the **highest** degree on the CV.
- Keep `skills` to the concrete, gradable ones actually stated on the CV (5–10).
- Include a few CVs that are intentionally hard (scanned/rotated, missing fields) to exercise the failure modes.

## Run
See `docs/uat/validation-runbook.md` → "Parse accuracy".

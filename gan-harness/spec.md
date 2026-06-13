# Spec — CP Axtra Careers Portal + Admin System (design redesign)

Redesign a **careers portal** (public) and **admin system** (internal) for **CP Axtra**, a
national-level Thai retail organization. Deliverable for this GAN loop: a **self-contained,
multi-page static HTML/CSS prototype** under `gan-harness/prototype/` that demonstrates the full
visual design system across every page below. Visual excellence is the primary goal.

## Brand Identity (non-negotiable)
- **Primary blue `#0B47B8`** — dominant: main UI, CTA buttons, sidebar, headers.
- **Accent yellow `#FFC02E`** — intentional accent only: highlights, badges, active states, hover accents. Never the dominant color.
- Tone: authoritative yet modern, trustworthy, forward-looking — fits a leading Thai enterprise of CP Axtra's scale.
- A restrained multicolour dot motif may be used as a brand signature (sparingly).

## Typography
- **Thai everywhere** — all primary copy in Thai (with English where natural / bilingual labels).
- Thai font: **Noto Sans Thai** or **IBM Plex Sans Thai** (Google Fonts).
- English/Latin + numerals: **Inter** or **IBM Plex Sans**.
- Hierarchy must read cleanly in Thai and English simultaneously (line-height tuned for Thai ascenders/descenders).

## Careers website (public, mobile-first, responsive)
1. **Home / Hero** — headline + job search bar (keyword + location), trust signals, "invests in people" feel.
2. **Job listing** — results grid/list with filters: department, location, employment type; result count; sort.
3. **Job detail** — title, location, type, responsibilities, qualifications, benefits, sticky Apply CTA.
4. **Application form** — multi-section (personal info, resume upload, consent), clear progress, accessible.
5. **About CP Axtra careers** — culture, values, why join, growth — human and approachable.

## Admin system (internal, desktop-optimized, `#0B47B8` sidebar)
6. **Dashboard** — KPI overview (open positions, applicants, pipeline stats) + charts; readable, uncluttered.
7. **Job posting management** — list with status (draft/published), plus create/edit form (publish flow).
8. **Applicant tracking** — applicants per job posting, pipeline/kanban or ranked table.
9. **Applicant detail** — profile + resume + status management (move through stages).
10. **User & role management** — users table, roles, permissions.

## Design direction
- Modern **Thai enterprise** aesthetic — NOT generic SaaS, NOT old-government. Clean type, generous whitespace.
- `#0B47B8` dominant, `#FFC02E` as a deliberate accent only.
- Data-heavy admin pages stay readable and uncluttered (Linear / Notion information density, `#0B47B8` sidebar).
- Careers site mobile-first; admin desktop-first.
- Polish target: **Workday-level**, adapted for Thai enterprise sensibility.

## Inspiration
- Careers: approachable, human, professional — feels like CP Axtra invests in people.
- Admin: Linear/Notion density with a `#0B47B8` sidebar.
- Overall: Workday-grade polish, Thai enterprise.

## Prototype constraints (for the GAN loop)
- All pages live under `gan-harness/prototype/` as static `.html` + shared `styles.css` (+ optional small JS for tabs/menus). No build step.
- Served via `python3 -m http.server` so the evaluator can screenshot via Playwright.
- A `prototype/index.html` links every page (careers + admin) so the evaluator can navigate.
- Real Thai copy (no lorem). Realistic CP Axtra retail roles (แคชเชียร์, ผู้จัดการสาขา, พนักงานคลัง, ฯลฯ).
- Animate only compositor-friendly props (transform/opacity). Respect prefers-reduced-motion.

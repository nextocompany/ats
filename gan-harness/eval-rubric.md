# Eval Rubric — CP Axtra Careers + Admin (design GAN)

Score each dimension 0–10. Weighted total = Σ(score × weight). **Pass threshold: 8.0.**
Evaluate from the live prototype screenshots (1440 / 768 / 320) across ALL pages.

### Design Quality (weight: 0.35)
- Strong visual hierarchy, scale contrast, generous whitespace.
- `#0B47B8` reads as the dominant brand; `#FFC02E` used as a deliberate accent only (not overused).
- Consistent, intentional spacing/rhythm, radii, shadows, surfaces across all pages.
- Careers feels approachable/human; admin feels dense-but-uncluttered (Linear/Notion-grade).
- Would this look believable in a real CP Axtra product screenshot? Workday-level polish?

### Originality (weight: 0.30)
- Not generic SaaS, not old-government, not a default Tailwind/Bootstrap template.
- Distinctive, opinionated layout choices; editorial/bento/grid-breaking where it fits.
- A memorable brand signature (e.g., the dot motif, a custom hero, a unique pipeline view).
- Creative leaps that feel specific to CP Axtra — would a design jury notice it?

### Craft (weight: 0.25)
- Thai typography done right (Noto/IBM Plex Thai, line-height, no clipping), bilingual hierarchy clean.
- Responsive: careers mobile-first works at 320/768/1440; admin polished at desktop, degrades gracefully.
- Designed hover/focus/active states; accessible contrast; alignment precision; no overflow/clipping.
- Real Thai copy, realistic CP Axtra retail roles; no lorem; no broken images/icons.

### Functionality (weight: 0.10)
- All 10 pages present and navigable from index; nav/tabs/menus work.
- No console errors; forms/filters visually complete; links resolve.

## Per-iteration output (evaluator writes feedback/feedback-NNN.md)
- Weighted total + per-dimension scores.
- Top 3 highest-impact improvements for the next iteration (specific, page-level).
- What to KEEP (so the generator doesn't regress strong areas).
- PASS/CONTINUE verdict (PASS at ≥ 8.0).

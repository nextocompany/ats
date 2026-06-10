# Evaluation Rubric — HR Dashboard redesign (design-weighted)

Score each dimension **0–10**, weighted total. **Pass threshold: 7.5.**
Evaluate from real screenshots of the running local dashboard at desktop (1440) AND
mobile (390), across the key pages. Judge "would this win a design award / look
believable in a premium product screenshot?"

### Design Quality (weight: 0.35)
Hierarchy through scale contrast; intentional spacing rhythm; refined typography + pairing;
cohesive color use (CP Axtra blue/yellow, not decorative noise); polished data-viz; overall
composition. Penalize generic template look, uniform-padding card grids, weak hierarchy.

### Originality (weight: 0.30)
Distinctive, opinionated layout and details — editorial/bento composition, the dot motif used
with intent, signature interactions. Reward creative leaps that still read as enterprise HR.
Penalize "default shadcn/Tailwind dashboard".

### Craft (weight: 0.25)
Pixel-level execution: alignment, consistent radii/elevation/spacing tokens, designed
hover/focus/active/empty/loading states, responsive correctness (no overflow at 320/768/1440),
AA contrast + visible focus rings, compositor-friendly motion.

### Functionality (weight: 0.10)
Pages render without errors, routes/data intact, build green. (Lower weight — design mode.)

### HARD GATE — CP Axtra CI preserved
If the redesign abandons the CP Axtra identity (not clearly blue `#0B47B8` + yellow `#FFC02E`
+ white + dot motif), cap the total at **5.0** regardless of other scores and say so in feedback.

## Output format (the evaluator returns this)
- Per-dimension scores (0–10) + one-line justification each.
- Weighted total (2 decimals) and PASS/FAIL vs 7.5.
- CI gate: preserved? yes/no.
- **Top 3–5 concrete, actionable fixes** for the next generator iteration (page + what + why).

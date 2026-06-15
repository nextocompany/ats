"use client";

import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { LEVELS, levelLabel, type Level } from "@/lib/levels";

interface JobFiltersProps {
  query: string;
  levels: Level[];
  // Per-level counts (over the unfiltered set) shown beside each facet label.
  levelCounts: Record<string, number>;
  onQueryChange: (value: string) => void;
  onToggleLevel: (level: Level) => void;
  onClear: () => void;
  hasActiveFilters: boolean;
}

// JobFilters is the left-rail filter panel: a free-text search bound to the title
// query and a level facet (the only fields the public API exposes). Flat,
// hairline-framed, institutional — no decoration. On mobile it sits above the
// results (the parent collapses it).
export function JobFilters({
  query,
  levels,
  levelCounts,
  onQueryChange,
  onToggleLevel,
  onClear,
  hasActiveFilters,
}: JobFiltersProps) {
  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-2">
        <label htmlFor="job-search" className="text-xs font-semibold uppercase tracking-[0.14em] text-foreground">
          ค้นหาตำแหน่ง
        </label>
        <div className="relative">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
            className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-muted-foreground/60"
          >
            <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="1.75" />
            <path d="m20 20-3.5-3.5" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
          </svg>
          <Input
            id="job-search"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder="ชื่อตำแหน่ง เช่น แคชเชียร์"
            autoComplete="off"
            className="pl-10"
          />
        </div>
      </div>

      <fieldset className="flex flex-col gap-3">
        <legend className="mb-1 text-xs font-semibold uppercase tracking-[0.14em] text-foreground">
          ระดับตำแหน่ง
        </legend>
        <ul className="flex flex-col gap-0.5">
          {LEVELS.map((level) => {
            const checked = levels.includes(level);
            const count = levelCounts[level] ?? 0;
            return (
              <li key={level}>
                <label className="flex cursor-pointer items-center gap-3 rounded-lg px-2 py-2.5 transition-colors hover:bg-secondary">
                  <Checkbox
                    checked={checked}
                    onCheckedChange={() => onToggleLevel(level)}
                    aria-label={levelLabel(level)}
                  />
                  <span className="flex-1 text-sm text-foreground">{levelLabel(level)}</span>
                  <span className="num text-xs text-muted-foreground">{count}</span>
                </label>
              </li>
            );
          })}
        </ul>
      </fieldset>

      {hasActiveFilters ? (
        <button
          type="button"
          onClick={onClear}
          className="self-start text-sm font-medium text-primary underline-offset-4 transition-colors hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:rounded-lg"
        >
          ล้างตัวกรองทั้งหมด
        </button>
      ) : null}
    </div>
  );
}

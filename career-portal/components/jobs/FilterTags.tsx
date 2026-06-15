"use client";

import { levelLabel, type Level } from "@/lib/levels";

export interface FilterTag {
  // A stable key for the removable tag.
  key: string;
  label: string;
  onRemove: () => void;
}

interface FilterTagsProps {
  query: string;
  levels: Level[];
  onRemoveQuery: () => void;
  onRemoveLevel: (level: Level) => void;
}

// FilterTags renders the active filters as removable chips. Each chip is a button
// (≥the tap minimum via padding) with a quiet ✕; removing it updates the URL state.
export function FilterTags({ query, levels, onRemoveQuery, onRemoveLevel }: FilterTagsProps) {
  const tags: FilterTag[] = [];
  if (query.trim()) {
    tags.push({ key: "q", label: `“${query.trim()}”`, onRemove: onRemoveQuery });
  }
  for (const level of levels) {
    tags.push({ key: `level-${level}`, label: levelLabel(level), onRemove: () => onRemoveLevel(level) });
  }

  if (tags.length === 0) return null;

  return (
    <ul className="flex flex-wrap items-center gap-2">
      {tags.map((tag) => (
        <li key={tag.key}>
          <button
            type="button"
            onClick={tag.onRemove}
            className="group inline-flex items-center gap-1.5 rounded-full border border-line bg-card py-1.5 pl-3 pr-2.5 text-sm text-foreground transition-colors hover:border-foreground/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            aria-label={`ลบตัวกรอง ${tag.label}`}
          >
            {tag.label}
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
              className="text-muted-foreground transition-colors group-hover:text-foreground"
            >
              <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
        </li>
      ))}
    </ul>
  );
}

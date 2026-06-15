"use client";

import { useCallback, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { LEVELS, type Level } from "@/lib/levels";

export interface JobFilterState {
  query: string;
  levels: Level[];
}

function isLevel(value: string): value is Level {
  return (LEVELS as readonly string[]).includes(value);
}

// useJobFilters mirrors the jobs filter state to the URL search-params so it is
// shareable and back-button safe. `q` holds the free-text title search; `level`
// is a repeatable param for the level facet. Parsing on read + router.replace on
// write keeps the URL the single source of truth (no duplicated client state).
export function useJobFilters() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const query = searchParams.get("q") ?? "";
  const levels = useMemo<Level[]>(
    () => searchParams.getAll("level").filter(isLevel),
    [searchParams],
  );

  const commit = useCallback(
    (next: JobFilterState) => {
      const params = new URLSearchParams();
      if (next.query.trim()) params.set("q", next.query.trim());
      for (const level of next.levels) params.append("level", level);
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [pathname, router],
  );

  const setQuery = useCallback(
    (value: string) => commit({ query: value, levels }),
    [commit, levels],
  );

  const toggleLevel = useCallback(
    (level: Level) => {
      const next = levels.includes(level)
        ? levels.filter((l) => l !== level)
        : [...levels, level];
      commit({ query, levels: next });
    },
    [commit, levels, query],
  );

  const removeLevel = useCallback(
    (level: Level) => commit({ query, levels: levels.filter((l) => l !== level) }),
    [commit, levels, query],
  );

  const clearQuery = useCallback(() => commit({ query: "", levels }), [commit, levels]);

  const clearAll = useCallback(() => commit({ query: "", levels: [] }), [commit]);

  const hasActiveFilters = query.trim().length > 0 || levels.length > 0;

  return {
    query,
    levels,
    hasActiveFilters,
    setQuery,
    toggleLevel,
    removeLevel,
    clearQuery,
    clearAll,
  };
}

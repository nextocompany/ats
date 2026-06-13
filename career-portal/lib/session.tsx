"use client";

import { createContext, useContext } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError } from "./api";
import { getMe } from "./auth";
import type { Account } from "./types";

export const ME_QUERY_KEY = ["candidate-me"] as const;

interface CandidateSession {
  candidate: Account | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  // refresh re-reads /auth/me (e.g. after OAuth return or email verify).
  refresh: () => Promise<void>;
}

const SessionContext = createContext<CandidateSession | null>(null);

// CandidateSessionProvider exposes the logged-in candidate across the portal. It
// reads GET /auth/me once; a 401 is the normal logged-out state (not an error).
export function CandidateSessionProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const query = useQuery<Account | null>({
    queryKey: ME_QUERY_KEY,
    queryFn: async () => {
      try {
        return await getMe();
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null;
        throw err;
      }
    },
    staleTime: 30_000,
    retry: false,
  });

  const value: CandidateSession = {
    candidate: query.data ?? null,
    isAuthenticated: !!query.data,
    isLoading: query.isLoading,
    refresh: async () => {
      await qc.invalidateQueries({ queryKey: ME_QUERY_KEY });
    },
  };

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

// useCandidate returns the current session. Throws if used outside the provider.
export function useCandidate(): CandidateSession {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useCandidate must be used within CandidateSessionProvider");
  return ctx;
}

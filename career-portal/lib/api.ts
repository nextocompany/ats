// Envelope-aware API client for the public Career API. Unwraps {success,data,error,meta}
// and throws ApiError on failure. Every request sends credentials:'include' so the
// candidate session cookie (cp_session) rides along — required for membership +
// account-first apply. The cookie is cross-site in prod, so the backend sets it
// SameSite=None;Secure and CORS allows credentials.
import type { Envelope, Meta } from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function unwrap<T>(res: Response): Promise<{ data: T; meta?: Meta }> {
  let env: Envelope<T> | null = null;
  try {
    env = (await res.json()) as Envelope<T>;
  } catch {
    throw new ApiError(`Unexpected response (${res.status})`, res.status);
  }
  if (!res.ok || !env.success) {
    throw new ApiError(env?.error ?? `Request failed (${res.status})`, res.status);
  }
  return { data: env.data, meta: env.meta };
}

export const api = {
  get: async <T>(path: string): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, { credentials: "include" });
    return unwrap<T>(res);
  },
  // post sends a JSON body. Used by the AI pre-interview chat (token-gated) and
  // the membership endpoints (cookie session).
  post: async <T>(path: string, body?: unknown): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    return unwrap<T>(res);
  },
  // patch sends a JSON body (profile update).
  patch: async <T>(path: string, body?: unknown): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, {
      method: "PATCH",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    return unwrap<T>(res);
  },
  // postForm posts multipart/form-data (resume upload). The browser sets the
  // multipart boundary Content-Type automatically — do not set it manually.
  postForm: async <T>(
    path: string,
    form: FormData,
    headers?: Record<string, string>,
  ): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, {
      method: "POST",
      credentials: "include",
      body: form,
      headers,
    });
    return unwrap<T>(res);
  },
  // del sends a DELETE (resume removal).
  del: async <T>(path: string): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, { method: "DELETE", credentials: "include" });
    return unwrap<T>(res);
  },
};

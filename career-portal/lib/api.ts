// Envelope-aware API client for the public Career API. Unwraps {success,data,error,meta}
// and throws ApiError on failure. Public endpoints need no bearer token; apply sends a
// LINE id-token header and a multipart body (no JSON).
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
    const res = await fetch(`${BASE}${path}`);
    return unwrap<T>(res);
  },
  // post sends a JSON body. Used by the AI pre-interview chat (token-gated, no
  // bearer). Apply still uses postForm (multipart resume upload).
  post: async <T>(path: string, body?: unknown): Promise<{ data: T; meta?: Meta }> => {
    const res = await fetch(`${BASE}${path}`, {
      method: "POST",
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
    const res = await fetch(`${BASE}${path}`, { method: "POST", body: form, headers });
    return unwrap<T>(res);
  },
};

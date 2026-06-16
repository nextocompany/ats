// Envelope-aware API client for the Go backend. Unwraps {success,data,error,meta}
// and throws ApiError on failure. In dev the backend trusts a mock super_admin,
// so no bearer token is required. In Entra mode we attach the Entra ID token
// (aud = our client ID) as `Authorization: Bearer <idToken>`, which the Go API
// validates via OIDC discovery.
import type { Envelope, Meta } from "./types";
import { getIdToken, isEntraConfigured } from "./auth";

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

async function request<T>(method: string, path: string, body?: unknown): Promise<{ data: T; meta?: Meta }> {
  const headers: Record<string, string> = {};
  if (body) headers["Content-Type"] = "application/json";
  if (isEntraConfigured()) {
    const token = await getIdToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: Object.keys(headers).length ? headers : undefined,
    body: body ? JSON.stringify(body) : undefined,
    credentials: "include",
  });

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

// requestForm posts a multipart FormData body (file uploads). The browser sets the
// Content-Type boundary, so we must NOT set it ourselves. Same auth + envelope
// handling as `request`.
async function requestForm<T>(path: string, form: FormData): Promise<{ data: T; meta?: Meta }> {
  const headers: Record<string, string> = {};
  if (isEntraConfigured()) {
    const token = await getIdToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: Object.keys(headers).length ? headers : undefined,
    body: form,
    credentials: "include",
  });

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
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  postForm: <T>(path: string, form: FormData) => requestForm<T>(path, form),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  del: <T>(path: string) => request<T>("DELETE", path),
};

// downloadFile fetches a non-JSON endpoint (e.g. a CSV export) with the same auth
// as `request` and triggers a browser download. Used for endpoints that stream a
// file attachment rather than the JSON envelope, so they can't go through `api`.
export async function downloadFile(path: string, filename: string): Promise<void> {
  const headers: Record<string, string> = {};
  if (isEntraConfigured()) {
    const token = await getIdToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE}${path}`, { headers, credentials: "include" });
  if (!res.ok) {
    throw new ApiError(`Download failed (${res.status})`, res.status);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename.replace(/[/\\]/g, "_"); // never let a path separator through

  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function buildQuery(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "" && v !== null) sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

import { z } from "zod";

import { API_BASE_URL } from "./config";
import { healthResponseSchema, type HealthResponse } from "@/types/health";

// getJSON fetches a path from the Go backend and validates the response body
// against the given zod schema, throwing on a non-OK status or schema mismatch.
async function getJSON<T>(path: string, schema: z.ZodType<T>): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`${path} responded ${res.status}`);
  }
  return schema.parse(await res.json());
}

// fetchHealth returns the backend health status, or null if the backend is
// unreachable (so the UI can render a friendly offline state).
export async function fetchHealth(): Promise<HealthResponse | null> {
  try {
    return await getJSON("/healthz", healthResponseSchema);
  } catch {
    return null;
  }
}

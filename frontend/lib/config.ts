// Frontend configuration. The model name is mirrored here only for display;
// the backend's internal/config is the single source of truth for LLM calls.

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export const MODEL_NAME = "claude-sonnet-4-6";

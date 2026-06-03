import { z } from "zod";

// Schema for the backend's GET /healthz response. All API responses are
// validated at the client boundary with zod (see CLAUDE.md).
export const healthResponseSchema = z.object({
  status: z.string(),
  db: z.string(),
});

export type HealthResponse = z.infer<typeof healthResponseSchema>;

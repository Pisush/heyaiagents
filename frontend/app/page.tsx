import { fetchHealth } from "@/lib/api";
import { MODEL_NAME } from "@/lib/config";

// Server component: fetches backend health at request time so the landing page
// proves the Next.js frontend and the Go backend can talk to each other.
export default async function Home() {
  const health = await fetchHealth();
  const online = health?.status === "ok" && health?.db === "ok";

  return (
    <main className="min-h-screen flex flex-col items-center justify-center gap-6 p-8">
      <h1 className="text-3xl font-bold">HeyAI Agents</h1>
      <p className="text-center max-w-md text-gray-600 dark:text-gray-300">
        Build a personal AI agent that virtually attends the conference on your
        behalf — coverage, networking, and recognition.
      </p>

      <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-4 w-full max-w-sm">
        <h2 className="font-semibold mb-2">Backend status</h2>
        <ul className="text-sm space-y-1">
          <li className="flex justify-between">
            <span>API</span>
            <span className={online ? "text-green-600" : "text-red-600"}>
              {online ? "online" : "offline"}
            </span>
          </li>
          <li className="flex justify-between">
            <span>Database</span>
            <span className={health?.db === "ok" ? "text-green-600" : "text-red-600"}>
              {health?.db ?? "unknown"}
            </span>
          </li>
          <li className="flex justify-between">
            <span>Model</span>
            <span className="text-gray-500">{MODEL_NAME}</span>
          </li>
        </ul>
      </div>
    </main>
  );
}

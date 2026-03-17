"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { fetchLogEvents, type LogEvent } from "@/lib/logs-api";

const DEFAULT_BACKEND_URL = "http://localhost:8088";
const DEFAULT_LIMIT = 100;
const POLL_INTERVAL_MS = 2000;
const MAX_EVENTS = 1000;

function formatTimestamp(timestamp: string): string {
  if (!timestamp) {
    return "-";
  }

  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return timestamp;
  }

  return date.toLocaleString();
}

function mergeEvents(existing: LogEvent[], incoming: LogEvent[]): LogEvent[] {
  if (incoming.length === 0) {
    return existing;
  }

  const seen = new Set(existing.map((event) => event.sequence));
  const merged = [...existing];

  for (const event of incoming) {
    if (!seen.has(event.sequence)) {
      merged.push(event);
      seen.add(event.sequence);
    }
  }

  merged.sort((a, b) => a.sequence - b.sequence);

  if (merged.length > MAX_EVENTS) {
    return merged.slice(merged.length - MAX_EVENTS);
  }

  return merged;
}

function formatValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }

  if (value === undefined) {
    return "-";
  }

  return JSON.stringify(value, null, 2);
}

export default function Home() {
  const [backendUrl, setBackendUrl] = useState(DEFAULT_BACKEND_URL);
  const [limit, setLimit] = useState(DEFAULT_LIMIT);
  const [events, setEvents] = useState<LogEvent[]>([]);
  const [latestSequence, setLatestSequence] = useState(0);
  const [isPolling, setIsPolling] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchNextPage = useCallback(async () => {
    setIsLoading(true);

    try {
      const result = await fetchLogEvents({
        backendUrl,
        since: latestSequence,
        limit,
      });

      setEvents((current) => mergeEvents(current, result.events));
      setLatestSequence((current) => {
        const maxFromEvents = result.events.reduce(
          (maxValue, event) => Math.max(maxValue, event.sequence),
          0
        );

        return Math.max(current, result.latestSequence, maxFromEvents);
      });
      setLastUpdated(new Date().toISOString());
      setError(null);
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "poll failed");
    } finally {
      setIsLoading(false);
    }
  }, [backendUrl, latestSequence, limit]);

  useEffect(() => {
    if (!isPolling) {
      return;
    }

    const timer = window.setInterval(() => {
      void fetchNextPage();
    }, POLL_INTERVAL_MS);

    return () => window.clearInterval(timer);
  }, [fetchNextPage, isPolling]);

  const statusText = useMemo(() => {
    const pollingState = isPolling ? "running" : "stopped";
    const updateText = lastUpdated
      ? `last update ${new Date(lastUpdated).toLocaleTimeString()}`
      : "not polled yet";

    return `Polling ${pollingState} • latest sequence ${latestSequence} • ${events.length} events • ${updateText}`;
  }, [events.length, isPolling, lastUpdated, latestSequence]);

  return (
    <main className="min-h-screen bg-slate-950 p-6 text-slate-100">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4">
        <header className="space-y-1">
          <h1 className="text-2xl font-semibold">Realtime Backend Log Monitor</h1>
          <p className="text-sm text-slate-300">
            Polls <code>/v1/logs/events</code> incrementally for quick backend
            testing.
          </p>
        </header>

        <section className="grid gap-3 rounded-lg border border-slate-700 bg-slate-900 p-4 md:grid-cols-[1fr_120px_auto]">
          <label className="flex flex-col gap-1 text-sm">
            Backend URL
            <input
              className="rounded border border-slate-600 bg-slate-950 px-3 py-2 text-sm"
              value={backendUrl}
              onChange={(event) => setBackendUrl(event.target.value)}
              placeholder="http://localhost:8088"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            Limit
            <input
              className="rounded border border-slate-600 bg-slate-950 px-3 py-2 text-sm"
              type="number"
              min={1}
              max={2000}
              value={limit}
              onChange={(event) => {
                const value = Number(event.target.value);
                setLimit(Number.isFinite(value) && value > 0 ? value : DEFAULT_LIMIT);
              }}
            />
          </label>

          <div className="flex flex-wrap items-end gap-2">
            <button
              className="rounded bg-emerald-600 px-3 py-2 text-sm font-medium text-white hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
              type="button"
              onClick={() => {
                setIsPolling(true);
                void fetchNextPage();
              }}
              disabled={isLoading}
            >
              Start
            </button>
            <button
              className="rounded bg-amber-600 px-3 py-2 text-sm font-medium text-white hover:bg-amber-500"
              type="button"
              onClick={() => setIsPolling(false)}
            >
              Stop
            </button>
            <button
              className="rounded bg-sky-700 px-3 py-2 text-sm font-medium text-white hover:bg-sky-600 disabled:cursor-not-allowed disabled:opacity-60"
              type="button"
              onClick={() => {
                void fetchNextPage();
              }}
              disabled={isLoading}
            >
              Poll now
            </button>
            <button
              className="rounded bg-slate-700 px-3 py-2 text-sm font-medium text-white hover:bg-slate-600"
              type="button"
              onClick={() => {
                setEvents([]);
                setLatestSequence(0);
                setError(null);
                setLastUpdated(null);
              }}
            >
              Clear
            </button>
          </div>
        </section>

        <section className="rounded-lg border border-slate-700 bg-slate-900 p-4 text-sm text-slate-200">
          <p>{statusText}</p>
          {error ? <p className="mt-1 text-rose-400">Error: {error}</p> : null}
        </section>

        <section className="space-y-3 rounded-lg border border-slate-700 bg-slate-900 p-4">
          {events.length === 0 ? (
            <p className="text-sm text-slate-400">No events yet.</p>
          ) : (
            <div className="max-h-[65vh] space-y-3 overflow-auto pr-1">
              {events.map((event) => (
                <article
                  key={event.sequence}
                  className="rounded border border-slate-700 bg-slate-950 p-3"
                >
                  <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                    <span className="font-mono text-slate-100">#{event.sequence}</span>
                    <span className="text-slate-300">{formatTimestamp(event.timestamp)}</span>
                    <span className="font-mono text-emerald-300">{event.type}</span>
                    <span className="font-mono text-slate-300">tool: {event.tool ?? "-"}</span>
                    <span className="font-mono text-slate-300">req: {event.requestId ?? "-"}</span>
                    <span className="font-mono text-slate-300">task: {event.taskId ?? "-"}</span>
                  </div>

                  {event.message ? (
                    <pre className="mb-2 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                      {event.message}
                    </pre>
                  ) : null}
                  {event.error ? (
                    <pre className="mb-2 whitespace-pre-wrap break-words rounded bg-rose-950/60 p-2 font-mono text-xs text-rose-200">
                      {event.error}
                    </pre>
                  ) : null}

                  {event.metadata !== undefined ? (
                    <details className="mb-2" open>
                      <summary className="cursor-pointer text-xs text-slate-300">
                        Metadata
                      </summary>
                      <pre className="mt-1 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                        {formatValue(event.metadata)}
                      </pre>
                    </details>
                  ) : null}

                  {event.input !== undefined ? (
                    <details className="mb-2" open>
                      <summary className="cursor-pointer text-xs text-slate-300">
                        Input
                      </summary>
                      <pre className="mt-1 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                        {formatValue(event.input)}
                      </pre>
                    </details>
                  ) : null}

                  {event.output !== undefined ? (
                    <details className="mb-2" open>
                      <summary className="cursor-pointer text-xs text-slate-300">
                        Output
                      </summary>
                      <pre className="mt-1 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                        {formatValue(event.output)}
                      </pre>
                    </details>
                  ) : null}

                  {event.payload !== undefined ? (
                    <details className="mb-2">
                      <summary className="cursor-pointer text-xs text-slate-300">
                        Legacy payload
                      </summary>
                      <pre className="mt-1 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                        {formatValue(event.payload)}
                      </pre>
                    </details>
                  ) : null}

                  <details open>
                    <summary className="cursor-pointer text-xs text-slate-300">
                      Full event JSON
                    </summary>
                    <pre className="mt-1 whitespace-pre-wrap break-words rounded bg-slate-900 p-2 font-mono text-xs text-slate-100">
                      {formatValue(event.raw)}
                    </pre>
                  </details>
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}

export interface LogEvent {
  sequence: number;
  timestamp: string;
  type: string;
  tool?: string;
  message?: string;
  error?: string;
  requestId?: string;
  taskId?: string;
  userId?: string;
  deviceId?: string;
  metadata?: unknown;
  input?: unknown;
  output?: unknown;
  payload?: unknown;
  raw: Record<string, unknown>;
}

export interface LogEventsResponse {
  events: LogEvent[];
  latestSequence: number;
}

interface RawLogEventsResponse {
  events?: unknown;
  latest_sequence?: unknown;
}

export interface FetchLogEventsParams {
  backendUrl: string;
  since: number;
  limit: number;
}

function toNumber(value: unknown, fallback = 0): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }

  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }

  return fallback;
}

function toOptionalString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() !== "" ? value : undefined;
}

function normalizeEvent(raw: unknown): LogEvent | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }

  const record = raw as Record<string, unknown>;
  return {
    sequence: toNumber(record.sequence),
    timestamp: typeof record.timestamp === "string" ? record.timestamp : "",
    type: typeof record.type === "string" ? record.type : "unknown",
    tool: toOptionalString(record.tool),
    message: toOptionalString(record.message),
    error: toOptionalString(record.error),
    requestId: toOptionalString(record.requestId),
    taskId: toOptionalString(record.taskId),
    userId: toOptionalString(record.userId),
    deviceId: toOptionalString(record.deviceId),
    metadata: record.metadata,
    input: record.input,
    output: record.output,
    payload: record.payload,
    raw: record,
  };
}

function extractErrorMessage(body: unknown): string {
  if (body && typeof body === "object" && "error" in body) {
    const error = (body as { error?: unknown }).error;
    if (typeof error === "string" && error.trim() !== "") {
      return error;
    }
  }

  return "Request failed";
}

export async function fetchLogEvents({
  backendUrl,
  since,
  limit,
}: FetchLogEventsParams): Promise<LogEventsResponse> {
  const params = new URLSearchParams({
    backendUrl,
    since: String(Math.max(0, since)),
    limit: String(Math.max(1, limit)),
  });

  const response = await fetch(`/api/logs/events?${params.toString()}`, {
    method: "GET",
    cache: "no-store",
  });

  const body = (await response.json().catch(() => ({}))) as RawLogEventsResponse;
  if (!response.ok) {
    throw new Error(extractErrorMessage(body));
  }

  const eventsRaw = Array.isArray(body.events) ? body.events : [];
  const events = eventsRaw
    .map(normalizeEvent)
    .filter((event): event is LogEvent => event !== null)
    .sort((a, b) => a.sequence - b.sequence);

  return {
    events,
    latestSequence: toNumber(body.latest_sequence),
  };
}

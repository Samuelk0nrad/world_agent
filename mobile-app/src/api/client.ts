export type AppMemoryEntry = {
  id: string;
  source: string;
  content: string;
  createdAt: string;
};

export type AppExtension = {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
};

export type AssistantStep = {
  name: string;
  detail: string;
};

export type AssistantResult = {
  reply: string;
  steps: AssistantStep[];
};

export type AppHealth = {
  isHealthy: boolean;
};

type ApiOptions = {
  backendUrl?: string;
  googleAccessToken?: string;
};

type BackendAgentMessage = {
  role?: unknown;
  Role?: unknown;
  content?: unknown;
  Content?: unknown;
  text?: unknown;
  Text?: unknown;
};

type BackendAgentResponse = {
  prompt?: unknown;
  message?: BackendAgentMessage | string | null;
  messages?: Array<BackendAgentMessage | string> | null;
};

type BackendHealthResponse = {
  status?: unknown;
};

type BackendErrorPayload = {
  err?: unknown;
  error?: unknown;
};

export type MemoryEntry = AppMemoryEntry;
export type Extension = AppExtension;
export type AgentStep = AssistantStep;
export type AgentResult = AssistantResult;
export type HealthzResponse = AppHealth;

const DEFAULT_BASE_URL = process.env.EXPO_PUBLIC_AGENT_BACKEND_URL ?? "http://localhost:8080";

function resolveBaseUrl(backendUrl?: string): string {
  const base = (backendUrl ?? DEFAULT_BASE_URL).trim();
  return base.endsWith("/") ? base.slice(0, -1) : base;
}

async function parseErrorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as BackendErrorPayload;
    const err = typeof data.err === "string" ? data.err.trim() : "";
    if (err) {
      return err;
    }
    const error = typeof data.error === "string" ? data.error.trim() : "";
    if (error) {
      return error;
    }
  } catch {
    // fallback below
  }

  switch (response.status) {
    case 400:
      return "Bad request.";
    case 401:
      return "Unauthorized.";
    case 403:
      return "Forbidden.";
    case 404:
      return "Not found.";
    case 408:
      return "Request timeout.";
    case 429:
      return "Too many requests.";
    case 500:
      return "Internal server error.";
    case 502:
      return "Bad gateway.";
    case 503:
      return "Service unavailable.";
    case 504:
      return "Gateway timeout.";
    default:
      return `Request failed (${response.status}).`;
  }
}

function mapHealthResponse(response: BackendHealthResponse): AppHealth {
  return {
    isHealthy: response.status === "ok",
  };
}

function readTextValue(value: unknown): string {
  if (typeof value === "string") {
    const normalized = value.trim();
    if (normalized) {
      return normalized;
    }
  }
  return "";
}

function readMessageRole(message: BackendAgentMessage | string): string {
  if (typeof message === "string") {
    return "assistant";
  }
  return readTextValue(message.role) || readTextValue(message.Role) || "assistant";
}

function readMessageContent(message: BackendAgentMessage | string | null | undefined): string {
  if (typeof message === "string") {
    return readTextValue(message);
  }
  if (!message || typeof message !== "object") {
    return "";
  }

  return (
    readTextValue(message.content) ||
    readTextValue(message.Content) ||
    readTextValue(message.text) ||
    readTextValue(message.Text)
  );
}

function mapAgentResponse(response: BackendAgentResponse): AssistantResult {
  const items = Array.isArray(response.messages) ? response.messages : [];
  const steps: AssistantStep[] = items
    .map((item, index) => {
      const detail = readMessageContent(item);
      if (!detail) {
        return null;
      }

      return {
        name: `${readMessageRole(item)}-${index + 1}`,
        detail,
      };
    })
    .filter((step): step is AssistantStep => step !== null);

  const reply =
    readMessageContent(response.message) ||
    (items.length > 0 ? readMessageContent(items[items.length - 1]) : "");

  if (!reply) {
    throw new Error("Backend agent response is invalid.");
  }

  return { reply, steps };
}

export async function addMemory(content: string, options?: ApiOptions): Promise<AppMemoryEntry> {
  void content;
  void options;
  throw new Error("Memory API is not supported yet by backend /api contract.");
}

export async function listExtensions(options?: ApiOptions): Promise<AppExtension[]> {
  void options;
  throw new Error("Extensions API is not supported yet by backend /api contract.");
}

export async function setExtensionEnabled(
  id: string,
  enabled: boolean,
  options?: ApiOptions,
): Promise<AppExtension> {
  void id;
  void enabled;
  void options;
  throw new Error("Extensions API is not supported yet by backend /api contract.");
}

export async function runAgent(message: string, sessionId = 4, options?: ApiOptions): Promise<AssistantResult> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/api/agent`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      prompt: message,
      sessionId,
    }),
  });

  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }

  const data = (await response.json()) as BackendAgentResponse;
  return mapAgentResponse(data);
}

export async function checkBackendHealth(options?: ApiOptions): Promise<AppHealth> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/api/health`);
  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }

  const data = (await response.json()) as BackendHealthResponse;
  if (!data || data.status !== "ok") {
    throw new Error("Backend health response is invalid.");
  }

  return mapHealthResponse(data);
}

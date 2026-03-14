export type MemoryEntry = {
  id: string;
  source: string;
  content: string;
  createdAt: string;
};

export type Extension = {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
};

export type AgentStep = {
  name: string;
  detail: string;
};

export type AgentResult = {
  reply: string;
  steps: AgentStep[];
};

type ApiOptions = {
  backendUrl?: string;
  googleAccessToken?: string;
};

const DEFAULT_BASE_URL = process.env.EXPO_PUBLIC_AGENT_BACKEND_URL ?? "http://localhost:8088";

function resolveBaseUrl(backendUrl?: string): string {
  const base = (backendUrl ?? DEFAULT_BASE_URL).trim();
  return base.endsWith("/") ? base.slice(0, -1) : base;
}

async function parseErrorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
    if (data.error) {
      return data.error;
    }
  } catch {
    // fallback below
  }
  return `Request failed: ${response.status}`;
}

export async function addMemory(content: string, options?: ApiOptions): Promise<MemoryEntry> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/v1/memory`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ source: "mobile", content }),
  });

  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }

  const data = (await response.json()) as { entry: MemoryEntry };
  return data.entry;
}

export async function listExtensions(options?: ApiOptions): Promise<Extension[]> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/v1/extensions`);
  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }
  const data = (await response.json()) as {
    extensions: Extension[];
  };
  return data.extensions;
}

export async function setExtensionEnabled(id: string, enabled: boolean, options?: ApiOptions): Promise<Extension> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/v1/extensions/${id}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ enabled }),
  });

  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }
  const data = (await response.json()) as { extension: Extension };
  return data.extension;
}

export async function runAgent(message: string, maxSteps = 4, options?: ApiOptions): Promise<AgentResult> {
  const response = await fetch(`${resolveBaseUrl(options?.backendUrl)}/v1/agent/run`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      message,
      maxSteps,
      googleAccessToken: options?.googleAccessToken?.trim() || undefined,
    }),
  });

  if (!response.ok) {
    throw new Error(await parseErrorMessage(response));
  }
  const data = (await response.json()) as { result: AgentResult };
  return data.result;
}

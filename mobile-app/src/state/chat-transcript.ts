import * as FileSystem from "expo-file-system/legacy";

export type ChatMessage = {
  role: "user" | "assistant";
  text: string;
};

type TranscriptPayload = {
  version: number;
  messages: ChatMessage[];
};

export type ChatTranscriptLoadResult = {
  messages: ChatMessage[];
  warning?: string;
};

const TRANSCRIPT_VERSION = 1;
const STORAGE_FILE_PREFIX = "assistant-chat-session-";
const STORAGE_KEY_PREFIX = "worldagent-assistant-chat-session-";

function getStorageFileUri(sessionId: number): string | null {
  if (!FileSystem.documentDirectory) {
    return null;
  }

  return `${FileSystem.documentDirectory}${STORAGE_FILE_PREFIX}${sessionId}.json`;
}

function getStorageKey(sessionId: number): string {
  return `${STORAGE_KEY_PREFIX}${sessionId}`;
}

function getLocalStorage(): {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
  removeItem: (key: string) => void;
} | null {
  const candidate = (globalThis as { localStorage?: unknown }).localStorage;
  if (!candidate || typeof candidate !== "object") {
    return null;
  }

  const storage = candidate as {
    getItem?: unknown;
    setItem?: unknown;
    removeItem?: unknown;
  };

  if (
    typeof storage.getItem !== "function" ||
    typeof storage.setItem !== "function" ||
    typeof storage.removeItem !== "function"
  ) {
    return null;
  }

  return {
    getItem: storage.getItem as (key: string) => string | null,
    setItem: storage.setItem as (key: string, value: string) => void,
    removeItem: storage.removeItem as (key: string) => void,
  };
}

function normalizeMessages(value: unknown): ChatMessage[] | null {
  if (!Array.isArray(value)) {
    return null;
  }

  const normalized: ChatMessage[] = [];
  for (const item of value) {
    if (!item || typeof item !== "object") {
      return null;
    }

    const candidate = item as {
      role?: unknown;
      text?: unknown;
    };

    if ((candidate.role !== "user" && candidate.role !== "assistant") || typeof candidate.text !== "string") {
      return null;
    }

    const text = candidate.text.trim();
    if (!text) {
      continue;
    }

    normalized.push({ role: candidate.role, text });
  }

  return normalized;
}

function parsePayload(raw: string): ChatTranscriptLoadResult {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { messages: [] };
  }

  try {
    const payload = JSON.parse(trimmed) as Partial<TranscriptPayload> | unknown;
    if (!payload || typeof payload !== "object") {
      return {
        messages: [],
        warning: "Saved conversation data was invalid and has been reset.",
      };
    }

    const candidate = payload as Partial<TranscriptPayload>;
    if (candidate.version !== TRANSCRIPT_VERSION) {
      return {
        messages: [],
        warning: "Saved conversation version changed. Starting with a fresh chat.",
      };
    }

    const messages = normalizeMessages(candidate.messages);
    if (messages === null) {
      return {
        messages: [],
        warning: "Saved conversation data was malformed and has been reset.",
      };
    }

    return { messages };
  } catch {
    return {
      messages: [],
      warning: "Saved conversation data could not be read and has been reset.",
    };
  }
}

function serializePayload(messages: ChatMessage[]): string {
  return JSON.stringify({
    version: TRANSCRIPT_VERSION,
    messages,
  } satisfies TranscriptPayload);
}

function formatError(prefix: string, error: unknown): string {
  if (error instanceof Error && error.message.trim()) {
    return `${prefix}: ${error.message}`;
  }
  return `${prefix}.`;
}

export async function loadChatTranscript(sessionId: number): Promise<ChatTranscriptLoadResult> {
  const fileUri = getStorageFileUri(sessionId);
  const key = getStorageKey(sessionId);

  if (fileUri) {
    try {
      const info = await FileSystem.getInfoAsync(fileUri);
      if (!info.exists) {
        return { messages: [] };
      }

      const raw = await FileSystem.readAsStringAsync(fileUri);
      const parsed = parsePayload(raw);
      if (parsed.warning) {
        await FileSystem.deleteAsync(fileUri, { idempotent: true });
      }

      return parsed;
    } catch (error) {
      return {
        messages: [],
        warning: formatError("Unable to restore saved conversation", error),
      };
    }
  }

  const localStorage = getLocalStorage();
  if (!localStorage) {
    return {
      messages: [],
      warning: "Local transcript storage is unavailable on this platform.",
    };
  }

  try {
    const raw = localStorage.getItem(key) ?? "";
    const parsed = parsePayload(raw);
    if (parsed.warning) {
      localStorage.removeItem(key);
    }

    return parsed;
  } catch (error) {
    return {
      messages: [],
      warning: formatError("Unable to restore saved conversation", error),
    };
  }
}

export async function persistChatTranscript(sessionId: number, messages: ChatMessage[]): Promise<string | null> {
  const fileUri = getStorageFileUri(sessionId);
  const key = getStorageKey(sessionId);
  const payload = serializePayload(messages);

  if (fileUri) {
    try {
      await FileSystem.writeAsStringAsync(fileUri, payload);
      return null;
    } catch (error) {
      return formatError("Unable to save conversation locally", error);
    }
  }

  const localStorage = getLocalStorage();
  if (!localStorage) {
    return null;
  }

  try {
    localStorage.setItem(key, payload);
    return null;
  } catch (error) {
    return formatError("Unable to save conversation locally", error);
  }
}

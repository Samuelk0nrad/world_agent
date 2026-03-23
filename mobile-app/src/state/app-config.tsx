import AsyncStorage from "@react-native-async-storage/async-storage";
import { createContext, PropsWithChildren, useContext, useEffect, useMemo, useState } from "react";

type AppConfig = {
  backendUrl: string;
  googleAccessToken: string;
  sessionId: number;
  setBackendUrl: (value: string) => void;
  setGoogleAccessToken: (value: string) => void;
  setSessionId: (value: number) => void;
};

const DEFAULT_BACKEND_URL = process.env.EXPO_PUBLIC_AGENT_BACKEND_URL ?? "http://localhost:8080";
const DEFAULT_SESSION_ID = 4;
const APP_CONFIG_STORAGE_KEY = "@worldagent/app-config";

const AppConfigContext = createContext<AppConfig | null>(null);

function normalizeBackendUrl(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const normalized = value.trim().replace(/\/+$/, "");
  return normalized.length > 0 ? normalized : null;
}

function normalizeSessionId(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value <= 0) {
    return null;
  }
  return value;
}

function normalizeGoogleAccessToken(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  return value.trim();
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

export function AppConfigProvider({ children }: PropsWithChildren) {
  const [backendUrl, setBackendUrl] = useState(DEFAULT_BACKEND_URL);
  const [googleAccessToken, setGoogleAccessToken] = useState("");
  const [sessionId, setSessionId] = useState(DEFAULT_SESSION_ID);
  const [hasHydrated, setHasHydrated] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const hydrate = async () => {
      try {
        const raw = await AsyncStorage.getItem(APP_CONFIG_STORAGE_KEY);
        if (!raw) {
          if (isMounted) {
            setHasHydrated(true);
          }
          return;
        }

        let parsed: unknown;
        try {
          parsed = JSON.parse(raw);
        } catch (error) {
          console.warn("Failed to parse stored app config. Falling back to defaults.", error);
          await AsyncStorage.removeItem(APP_CONFIG_STORAGE_KEY);
          if (isMounted) {
            setHasHydrated(true);
          }
          return;
        }

        if (!isObject(parsed)) {
          console.warn("Stored app config has unexpected shape. Falling back to defaults.");
          await AsyncStorage.removeItem(APP_CONFIG_STORAGE_KEY);
          if (isMounted) {
            setHasHydrated(true);
          }
          return;
        }

        const nextBackendUrl = normalizeBackendUrl(parsed.backendUrl) ?? DEFAULT_BACKEND_URL;
        const nextSessionId = normalizeSessionId(parsed.sessionId) ?? DEFAULT_SESSION_ID;
        const nextGoogleAccessToken = normalizeGoogleAccessToken(parsed.googleAccessToken) ?? "";
        const hasInvalidBackendUrl =
          Object.prototype.hasOwnProperty.call(parsed, "backendUrl") && normalizeBackendUrl(parsed.backendUrl) === null;
        const hasInvalidSessionId =
          Object.prototype.hasOwnProperty.call(parsed, "sessionId") && normalizeSessionId(parsed.sessionId) === null;
        const hasInvalidGoogleAccessToken =
          Object.prototype.hasOwnProperty.call(parsed, "googleAccessToken") &&
          normalizeGoogleAccessToken(parsed.googleAccessToken) === null;

        if (!isMounted) {
          return;
        }

        if (hasInvalidBackendUrl || hasInvalidSessionId || hasInvalidGoogleAccessToken) {
          console.warn("Stored app config contained invalid values. Invalid fields were reset to safe defaults.");
        }

        setBackendUrl(nextBackendUrl);
        setSessionId(nextSessionId);
        setGoogleAccessToken(nextGoogleAccessToken);
        setHasHydrated(true);
      } catch (error) {
        console.warn("Failed to read app config from storage. Falling back to defaults.", error);
        if (isMounted) {
          setHasHydrated(true);
        }
      }
    };

    void hydrate();

    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    if (!hasHydrated) {
      return;
    }

    const persist = async () => {
      try {
        await AsyncStorage.setItem(
          APP_CONFIG_STORAGE_KEY,
          JSON.stringify({
            backendUrl,
            googleAccessToken,
            sessionId,
          }),
        );
      } catch (error) {
        console.warn("Failed to persist app config.", error);
      }
    };

    void persist();
  }, [backendUrl, googleAccessToken, hasHydrated, sessionId]);

  const value = useMemo<AppConfig>(
    () => ({
      backendUrl,
      googleAccessToken,
      sessionId,
      setBackendUrl,
      setGoogleAccessToken,
      setSessionId,
    }),
    [backendUrl, googleAccessToken, sessionId],
  );

  return <AppConfigContext.Provider value={value}>{children}</AppConfigContext.Provider>;
}

export function useAppConfig(): AppConfig {
  const context = useContext(AppConfigContext);
  if (!context) {
    throw new Error("useAppConfig must be used inside AppConfigProvider");
  }
  return context;
}

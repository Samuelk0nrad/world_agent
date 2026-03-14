import { createContext, PropsWithChildren, useContext, useMemo, useState } from "react";

type AppConfig = {
  backendUrl: string;
  googleAccessToken: string;
  setBackendUrl: (value: string) => void;
  setGoogleAccessToken: (value: string) => void;
};

const DEFAULT_BACKEND_URL = process.env.EXPO_PUBLIC_AGENT_BACKEND_URL ?? "http://localhost:8088";

const AppConfigContext = createContext<AppConfig | null>(null);

export function AppConfigProvider({ children }: PropsWithChildren) {
  const [backendUrl, setBackendUrl] = useState(DEFAULT_BACKEND_URL);
  const [googleAccessToken, setGoogleAccessToken] = useState("");

  const value = useMemo<AppConfig>(
    () => ({
      backendUrl,
      googleAccessToken,
      setBackendUrl,
      setGoogleAccessToken,
    }),
    [backendUrl, googleAccessToken],
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

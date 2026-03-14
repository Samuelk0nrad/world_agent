import { useState } from "react";
import { Pressable, SafeAreaView, StyleSheet, Switch, Text, TextInput, View } from "react-native";
import { checkBackendHealth } from "../src/api/client";
import { useAppConfig } from "../src/state/app-config";

export default function SettingsScreen() {
  const { backendUrl, googleAccessToken, setBackendUrl, setGoogleAccessToken } = useAppConfig();
  const [urlDraft, setUrlDraft] = useState(backendUrl);
  const [tokenDraft, setTokenDraft] = useState(googleAccessToken);
  const [status, setStatus] = useState("Ready");
  const [healthStatus, setHealthStatus] = useState("Not checked yet.");
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);

  const onSave = () => {
    const normalized = urlDraft.trim().replace(/\/+$/, "");
    if (!normalized) {
      setStatus("Backend URL is required.");
      return;
    }
    setBackendUrl(normalized);
    setGoogleAccessToken(tokenDraft.trim());
    setStatus("Saved. API settings are active now.");
  };

  const onCheckHealth = async () => {
    const normalized = urlDraft.trim().replace(/\/+$/, "");
    if (!normalized) {
      setHealthStatus("Backend URL is required.");
      return;
    }

    setIsCheckingHealth(true);
    try {
      const startedAt = Date.now();
      await checkBackendHealth({ backendUrl: normalized });
      const elapsedMs = Date.now() - startedAt;
      setHealthStatus(`Connected successfully (${elapsedMs} ms).`);
    } catch (error) {
      setHealthStatus(error instanceof Error ? `Connection failed: ${error.message}` : "Connection failed.");
    } finally {
      setIsCheckingHealth(false);
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <Text style={styles.title}>API Settings</Text>
      <Text style={styles.subtitle}>Configure backend API access for testing.</Text>

      <View style={styles.formCard}>
        <Text style={styles.fieldLabel}>Backend URL</Text>
        <TextInput
          style={styles.input}
          autoCapitalize="none"
          autoCorrect={false}
          value={urlDraft}
          onChangeText={setUrlDraft}
          placeholder="http://localhost:8088"
          placeholderTextColor="#64748b"
        />

        <Text style={styles.fieldLabel}>Google access token (optional, for Gmail tests)</Text>
        <TextInput
          style={styles.input}
          autoCapitalize="none"
          autoCorrect={false}
          value={tokenDraft}
          onChangeText={setTokenDraft}
          placeholder="ya29.a0..."
          placeholderTextColor="#64748b"
        />

        <Pressable style={styles.button} onPress={onSave}>
          <Text style={styles.buttonText}>Save settings</Text>
        </Pressable>

        <Pressable
          style={[styles.button, styles.secondaryButton, isCheckingHealth && styles.buttonDisabled]}
          onPress={() => void onCheckHealth()}
          disabled={isCheckingHealth}
        >
          <Text style={styles.buttonText}>{isCheckingHealth ? "Checking..." : "Check backend connection"}</Text>
        </Pressable>

        <Text style={styles.connectionStatus}>{healthStatus}</Text>
      </View>

      <View style={styles.item}>
        <Text style={styles.label}>Screen access enabled</Text>
        <Switch value={false} disabled />
      </View>

      <View style={styles.item}>
        <Text style={styles.label}>Conversation listening enabled</Text>
        <Switch value={false} disabled />
      </View>

      <Text style={styles.note}>{status}</Text>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 16,
    backgroundColor: "#0b1020",
    gap: 12,
  },
  title: {
    color: "#ffffff",
    fontSize: 24,
    fontWeight: "700",
  },
  subtitle: {
    color: "#cbd5e1",
    fontSize: 14,
  },
  formCard: {
    backgroundColor: "#111827",
    borderRadius: 10,
    borderWidth: 1,
    borderColor: "#334155",
    padding: 12,
    gap: 8,
  },
  fieldLabel: {
    color: "#93c5fd",
    fontSize: 12,
    fontWeight: "600",
  },
  input: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#334155",
    color: "#f8fafc",
    paddingHorizontal: 10,
    paddingVertical: 8,
  },
  button: {
    marginTop: 4,
    backgroundColor: "#2563eb",
    borderRadius: 8,
    paddingVertical: 10,
    alignItems: "center",
  },
  buttonText: {
    color: "#ffffff",
    fontWeight: "600",
  },
  secondaryButton: {
    backgroundColor: "#0f766e",
  },
  buttonDisabled: {
    opacity: 0.7,
  },
  connectionStatus: {
    color: "#bfdbfe",
    fontSize: 12,
  },
  item: {
    backgroundColor: "#111827",
    borderRadius: 10,
    borderWidth: 1,
    borderColor: "#334155",
    padding: 12,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  label: {
    color: "#f8fafc",
  },
  note: {
    color: "#93c5fd",
    fontSize: 12,
  },
});

import { useEffect, useState } from "react";
import { ScrollView, StyleSheet, Switch, Text, TextInput, View } from "react-native";
import { checkBackendHealth } from "../src/api/client";
import { Card, PrimaryButton, Screen, SectionHeader, StatusBanner } from "../src/components/ui";
import { useAppConfig } from "../src/state/app-config";
import { colors, radius, spacing, textVariants, typography } from "../src/theme";

function deriveTone(message: string): "info" | "success" | "warning" | "error" {
  const normalized = message.toLowerCase();

  if (normalized.startsWith("saved") || normalized.startsWith("connected successfully")) {
    return "success";
  }

  if (normalized.includes("must") || normalized.includes("required") || normalized.includes("failed")) {
    return "error";
  }

  if (normalized.includes("checking")) {
    return "info";
  }

  if (normalized.includes("valid")) {
    return "success";
  }

  return "info";
}

export default function SettingsScreen() {
  const { backendUrl, googleAccessToken, sessionId, setBackendUrl, setGoogleAccessToken, setSessionId } = useAppConfig();
  const [urlDraft, setUrlDraft] = useState(backendUrl);
  const [tokenDraft, setTokenDraft] = useState(googleAccessToken);
  const [sessionIdDraft, setSessionIdDraft] = useState(String(sessionId));
  const [status, setStatus] = useState("Ready");
  const [healthStatus, setHealthStatus] = useState("Not checked yet.");
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);

  useEffect(() => {
    setUrlDraft(backendUrl);
  }, [backendUrl]);

  useEffect(() => {
    setTokenDraft(googleAccessToken);
  }, [googleAccessToken]);

  useEffect(() => {
    setSessionIdDraft(String(sessionId));
  }, [sessionId]);

  const parseSessionId = (value: string): number | null => {
    const normalized = value.trim();
    if (!/^\d+$/.test(normalized)) {
      return null;
    }
    const parsed = Number(normalized);
    if (!Number.isSafeInteger(parsed) || parsed <= 0) {
      return null;
    }
    return parsed;
  };

  const onSave = () => {
    const normalized = urlDraft.trim().replace(/\/+$/, "");
    if (!normalized) {
      setStatus("Backend URL is required.");
      return;
    }
    const parsedSessionId = parseSessionId(sessionIdDraft);
    if (parsedSessionId === null) {
      setStatus("Session ID must be a positive integer.");
      return;
    }

    setBackendUrl(normalized);
    setSessionId(parsedSessionId);
    setGoogleAccessToken(tokenDraft.trim());
    setStatus("Saved. Backend and session settings are active now.");
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

  const parsedDraftSessionId = parseSessionId(sessionIdDraft);
  const normalizedUrlDraft = urlDraft.trim();
  const canSave = Boolean(normalizedUrlDraft) && parsedDraftSessionId !== null;
  const canCheckHealth = Boolean(normalizedUrlDraft) && !isCheckingHealth;
  const sessionValidationMessage =
    parsedDraftSessionId === null
      ? "Enter a positive integer session ID before saving."
      : `Session ID ${parsedDraftSessionId} is valid.`;

  return (
    <Screen>
      <SectionHeader title="Settings" subtitle="Manage backend connection, session context, and diagnostic actions." />

      <ScrollView contentContainerStyle={styles.content}>
        <Card style={styles.sectionCard}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>Connection configuration</Text>
            <Text style={styles.sectionSubtitle}>This base URL is used by Assistant and Extensions screens.</Text>
          </View>

          <View style={styles.fieldGroup}>
            <Text style={styles.fieldLabel}>Backend URL</Text>
            <Text style={styles.helperText}>Trailing slashes are removed when saving settings.</Text>
            <TextInput
              style={styles.input}
              autoCapitalize="none"
              autoCorrect={false}
              value={urlDraft}
              onChangeText={setUrlDraft}
              placeholder="http://localhost:8080"
              placeholderTextColor={colors.text.muted}
              accessibilityLabel="Backend URL"
              accessibilityHint="Enter your backend base URL, for example http://localhost:8080."
            />
          </View>
        </Card>

        <Card style={styles.sectionCard}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>Session and token</Text>
            <Text style={styles.sectionSubtitle}>Session ID controls conversation continuity. Token is optional for Gmail access.</Text>
          </View>

          <View style={styles.fieldGroup}>
            <Text style={styles.fieldLabel}>Session ID</Text>
            <TextInput
              style={styles.input}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="number-pad"
              value={sessionIdDraft}
              onChangeText={setSessionIdDraft}
              placeholder="1"
              placeholderTextColor={colors.text.muted}
              accessibilityLabel="Session ID"
              accessibilityHint="Enter a positive integer for session continuity."
            />
            <StatusBanner
              message={sessionValidationMessage}
              tone={parsedDraftSessionId === null ? "warning" : "success"}
              announce
            />
          </View>

          <View style={styles.fieldGroup}>
            <Text style={styles.fieldLabel}>Google access token (optional)</Text>
            <TextInput
              style={styles.input}
              autoCapitalize="none"
              autoCorrect={false}
              value={tokenDraft}
              onChangeText={setTokenDraft}
              placeholder="ya29.a0..."
              placeholderTextColor={colors.text.muted}
              accessibilityLabel="Google access token"
              accessibilityHint="Optional. Paste a Google access token to enable Gmail features."
            />
          </View>

          <View style={styles.capabilitiesGroup}>
            <Text style={styles.fieldLabel}>Access capabilities</Text>
            <View style={styles.capabilityRow}>
              <View style={styles.capabilityTextBlock}>
                <Text style={styles.capabilityLabel}>Screen access</Text>
                <Text style={styles.capabilityHint}>Reserved for future release.</Text>
              </View>
              <Switch
                value={false}
                disabled
                accessibilityRole="switch"
                accessibilityLabel="Screen access"
                accessibilityHint="Reserved for a future release."
                accessibilityState={{ disabled: true, checked: false }}
              />
            </View>
            <View style={styles.capabilityRow}>
              <View style={styles.capabilityTextBlock}>
                <Text style={styles.capabilityLabel}>Conversation listening</Text>
                <Text style={styles.capabilityHint}>Reserved for future release.</Text>
              </View>
              <Switch
                value={false}
                disabled
                accessibilityRole="switch"
                accessibilityLabel="Conversation listening"
                accessibilityHint="Reserved for a future release."
                accessibilityState={{ disabled: true, checked: false }}
              />
            </View>
          </View>
        </Card>

        <Card style={styles.sectionCard}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>Actions</Text>
            <Text style={styles.sectionSubtitle}>Save changes first, then optionally verify backend connectivity.</Text>
          </View>

          <View style={styles.actionGroup}>
            <PrimaryButton
              title="Save settings"
              onPress={onSave}
              disabled={!canSave}
              accessibilityHint={
                canSave ? "Saves backend URL, session ID, and token values." : "Enter a backend URL and valid session ID."
              }
            />
            <PrimaryButton
              title={isCheckingHealth ? "Checking..." : "Check backend health (/api/health)"}
              variant="secondary"
              onPress={() => void onCheckHealth()}
              disabled={!canCheckHealth}
              loading={isCheckingHealth}
              accessibilityHint={
                canCheckHealth
                  ? "Calls the backend health endpoint with the current URL."
                  : "Enter a backend URL or wait for the current check to finish."
              }
            />
          </View>

          <View style={styles.feedbackGroup}>
            <Text style={styles.feedbackLabel}>Settings status</Text>
            <StatusBanner message={status} tone={deriveTone(status)} announce />
          </View>
          <View style={styles.feedbackGroup}>
            <Text style={styles.feedbackLabel}>Health check</Text>
            <StatusBanner message={healthStatus} tone={deriveTone(healthStatus)} announce />
          </View>
        </Card>
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  content: {
    gap: spacing.md,
    paddingBottom: spacing.xl,
  },
  sectionCard: {
    gap: spacing.md,
  },
  sectionHeader: {
    gap: spacing.xs,
  },
  sectionTitle: {
    color: colors.text.primary,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
  },
  sectionSubtitle: {
    ...textVariants.body,
    color: colors.text.secondary,
  },
  fieldGroup: {
    gap: spacing.xs,
  },
  fieldLabel: {
    color: colors.accent.subtle,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  helperText: {
    ...textVariants.caption,
  },
  input: {
    borderRadius: radius.sm,
    borderWidth: 1,
    borderColor: colors.border.subtle,
    backgroundColor: colors.surface.elevated,
    color: colors.text.primary,
    paddingHorizontal: spacing.sm + 2,
    paddingVertical: spacing.sm,
  },
  capabilitiesGroup: {
    gap: spacing.sm,
  },
  capabilityRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: spacing.sm,
    minHeight: 44,
  },
  capabilityTextBlock: {
    flex: 1,
    gap: spacing.xs,
  },
  capabilityLabel: {
    ...textVariants.body,
    color: colors.text.primary,
    fontWeight: typography.weights.semibold,
  },
  capabilityHint: {
    ...textVariants.caption,
  },
  actionGroup: {
    gap: spacing.sm,
  },
  feedbackGroup: {
    gap: spacing.xs,
  },
  feedbackLabel: {
    color: colors.accent.subtle,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
});

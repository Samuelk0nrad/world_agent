import { useEffect, useMemo, useState } from "react";
import { ActivityIndicator, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { Extension, listExtensions, setExtensionEnabled } from "../src/api/client";
import { Card, Screen, SectionHeader, StatusBanner } from "../src/components/ui";
import { useAppConfig } from "../src/state/app-config";
import { colors, spacing, textVariants, typography } from "../src/theme";

const NOT_SUPPORTED_STATUS = "Not supported yet: this backend does not expose the Extensions API.";

type BannerTone = "info" | "success" | "warning" | "error";

function isNotSupportedError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }

  return error.message.toLowerCase().includes("not supported yet");
}

function getStatusTone(message: string, isUnsupported: boolean): BannerTone {
  if (isUnsupported) {
    return "warning";
  }

  const normalized = message.toLowerCase();
  if (normalized.includes("failed") || normalized.includes("error")) {
    return "error";
  }
  if (normalized.includes("loaded") || normalized.includes("enabled") || normalized.includes("disabled")) {
    return "success";
  }

  return "info";
}

function ExtensionCard({
  extension,
  onToggle,
  disabled,
  loading,
}: {
  extension: Extension;
  onToggle: (id: string, enabled: boolean) => void;
  disabled: boolean;
  loading: boolean;
}) {
  return (
    <Card style={styles.extensionCard}>
      <View style={styles.extensionHeaderRow}>
        <View style={styles.extensionTitleGroup}>
          <Text style={styles.extensionName}>{extension.name}</Text>
          <Text style={styles.extensionDescription}>{extension.description}</Text>
        </View>
        <View style={styles.switchArea}>
          {loading ? <ActivityIndicator size="small" color={colors.status.info} /> : null}
          <Switch
            value={extension.enabled}
            disabled={disabled}
            onValueChange={(value) => onToggle(extension.id, value)}
            accessibilityRole="switch"
            accessibilityLabel={`${extension.name} extension`}
            accessibilityHint={
              disabled ? "Disabled while updating extension state." : "Double tap to enable or disable this extension."
            }
            accessibilityState={{ disabled, checked: extension.enabled, busy: loading }}
          />
        </View>
      </View>
      <Text style={[styles.extensionState, extension.enabled ? styles.enabledText : styles.disabledText]}>
        {extension.enabled ? "Enabled" : "Disabled"}
      </Text>
    </Card>
  );
}

export default function ExtensionsScreen() {
  const { backendUrl } = useAppConfig();
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [status, setStatus] = useState("Checking extension availability...");
  const [isUnsupported, setIsUnsupported] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [pendingExtensionId, setPendingExtensionId] = useState<string | null>(null);

  const statusTone = useMemo(() => getStatusTone(status, isUnsupported), [status, isUnsupported]);

  const refresh = async () => {
    setStatus("Loading extensions...");
    setIsLoading(true);

    try {
      const items = await listExtensions({ backendUrl });
      setExtensions(items);
      setIsUnsupported(false);
      setStatus(`${items.length} extension(s) loaded`);
    } catch (error) {
      if (isNotSupportedError(error)) {
        setExtensions([]);
        setIsUnsupported(true);
        setStatus(NOT_SUPPORTED_STATUS);
        return;
      }

      setIsUnsupported(false);
      setStatus(error instanceof Error ? error.message : "Failed to load extensions.");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, [backendUrl]);

  const onToggle = async (id: string, enabled: boolean) => {
    setStatus("Updating extension...");
    setPendingExtensionId(id);

    try {
      const updated = await setExtensionEnabled(id, enabled, { backendUrl });
      setExtensions((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      setIsUnsupported(false);
      setStatus(`${updated.name} ${updated.enabled ? "enabled" : "disabled"}.`);
    } catch (error) {
      if (isNotSupportedError(error)) {
        setIsUnsupported(true);
        setStatus(NOT_SUPPORTED_STATUS);
        return;
      }

      setIsUnsupported(false);
      setStatus(error instanceof Error ? error.message : "Failed to update extension.");
    } finally {
      setPendingExtensionId(null);
    }
  };

  return (
    <Screen>
      <SectionHeader
        title="Extensions"
        subtitle="Discover and configure optional capabilities for your assistant runtime."
      />

      <Card style={styles.contextCard}>
        <View style={styles.contextRow}>
          <Text style={styles.contextLabel}>Connected backend</Text>
          <Text style={styles.contextValue}>{backendUrl}</Text>
        </View>
        <View style={styles.contextRow}>
          <Text style={styles.contextLabel}>Endpoint status</Text>
          <Text style={[styles.contextValue, isUnsupported && styles.warningText]}>
            {isUnsupported ? "Unsupported right now" : "Available when backend responds"}
          </Text>
        </View>
      </Card>

      <StatusBanner message={status} tone={statusTone} announce />

      <Card style={styles.catalogCard}>
        <View style={styles.catalogHeaderRow}>
          <Text style={styles.catalogTitle}>Extension catalog</Text>
          <View style={styles.catalogMetaRow}>
            {isLoading ? <ActivityIndicator size="small" color={colors.status.info} /> : null}
            <Text style={styles.catalogMeta}>
              {extensions.length} item{extensions.length === 1 ? "" : "s"}
            </Text>
          </View>
        </View>
        <Text style={styles.catalogDescription}>
          This area is ready for live extension rows once the backend endpoint is enabled.
        </Text>
      </Card>

      <ScrollView contentContainerStyle={styles.list}>
        {extensions.length === 0 ? (
          <Card style={styles.emptyCard}>
            <Text style={styles.emptyTitle}>
              {isUnsupported ? "Extensions endpoint unavailable" : "No extensions returned yet"}
            </Text>
            <Text style={styles.emptyBody}>
              {isUnsupported
                ? "The tab stays visible intentionally. Your backend does not expose extension listing or updates yet."
                : "No extension rows are available at the moment. They will appear here when the backend returns data."}
            </Text>
          </Card>
        ) : (
          extensions.map((extension) => (
            <ExtensionCard
              key={extension.id}
              extension={extension}
              onToggle={(id, enabled) => void onToggle(id, enabled)}
              disabled={isUnsupported || isLoading || pendingExtensionId !== null}
              loading={pendingExtensionId === extension.id}
            />
          ))
        )}
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  contextCard: {
    gap: spacing.sm,
  },
  contextRow: {
    gap: spacing.xs,
  },
  contextLabel: {
    color: colors.accent.subtle,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  contextValue: {
    ...textVariants.body,
    color: colors.text.primary,
  },
  warningText: {
    color: colors.status.warning,
  },
  catalogCard: {
    gap: spacing.xs,
  },
  catalogHeaderRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  catalogMetaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
  catalogTitle: {
    color: colors.text.primary,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
  },
  catalogMeta: {
    color: colors.text.muted,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.medium,
  },
  catalogDescription: {
    ...textVariants.body,
  },
  list: {
    gap: spacing.sm,
    paddingBottom: spacing.xl,
  },
  emptyCard: {
    gap: spacing.xs,
  },
  emptyTitle: {
    color: colors.text.primary,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
  },
  emptyBody: {
    ...textVariants.body,
  },
  extensionCard: {
    gap: spacing.sm,
  },
  extensionHeaderRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "flex-start",
    gap: spacing.md,
    minHeight: 44,
  },
  extensionTitleGroup: {
    flex: 1,
    gap: spacing.xs,
  },
  extensionName: {
    color: colors.text.primary,
    fontWeight: typography.weights.bold,
    fontSize: typography.sizes.md,
  },
  extensionDescription: {
    ...textVariants.body,
  },
  extensionState: {
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  enabledText: {
    color: colors.status.success,
  },
  disabledText: {
    color: colors.status.warning,
  },
  switchArea: {
    minHeight: 44,
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
});

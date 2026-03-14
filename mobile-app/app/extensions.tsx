import { useEffect, useState } from "react";
import { SafeAreaView, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { Extension, listExtensions, setExtensionEnabled } from "../src/api/client";
import { useAppConfig } from "../src/state/app-config";

export default function ExtensionsScreen() {
  const { backendUrl } = useAppConfig();
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [status, setStatus] = useState("Loading extensions...");

  const refresh = async () => {
    try {
      const items = await listExtensions({ backendUrl });
      setExtensions(items);
      setStatus(`${items.length} extension(s) loaded`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Failed to load");
    }
  };

  useEffect(() => {
    void refresh();
  }, [backendUrl]);

  const onToggle = async (id: string, enabled: boolean) => {
    setStatus("Updating extension...");
    try {
      const updated = await setExtensionEnabled(id, enabled, { backendUrl });
      setExtensions((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      setStatus(`${updated.name} ${updated.enabled ? "enabled" : "disabled"}.`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Failed to update");
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <Text style={styles.title}>Extensions</Text>
      <Text style={styles.subtitle}>Browse capabilities available to your assistant.</Text>
      <Text style={styles.subtitle}>Connected to: {backendUrl}</Text>
      <Text style={styles.status}>{status}</Text>

      <ScrollView contentContainerStyle={styles.list}>
        {extensions.map((ext) => (
          <View key={ext.id} style={styles.card}>
            <View style={styles.headerRow}>
              <Text style={styles.name}>{ext.name}</Text>
              <Switch value={ext.enabled} onValueChange={(value) => void onToggle(ext.id, value)} />
            </View>
            <Text style={styles.description}>{ext.description}</Text>
            <Text style={styles.badge}>{ext.enabled ? "Enabled" : "Disabled"}</Text>
          </View>
        ))}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 16,
    backgroundColor: "#0b1020",
    gap: 8,
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
  status: {
    color: "#93c5fd",
    fontSize: 13,
    marginBottom: 4,
  },
  list: {
    gap: 10,
    paddingBottom: 20,
  },
  card: {
    borderRadius: 10,
    backgroundColor: "#111827",
    borderColor: "#334155",
    borderWidth: 1,
    padding: 12,
    gap: 6,
  },
  headerRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  name: {
    color: "#f8fafc",
    fontWeight: "700",
    fontSize: 16,
  },
  description: {
    color: "#cbd5e1",
    fontSize: 13,
  },
  badge: {
    color: "#38bdf8",
    fontSize: 12,
    fontWeight: "600",
  },
});

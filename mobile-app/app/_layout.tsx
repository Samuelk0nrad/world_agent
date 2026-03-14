import { Tabs } from "expo-router";
import { AppConfigProvider } from "../src/state/app-config";

export default function RootLayout() {
  return (
    <AppConfigProvider>
      <Tabs screenOptions={{ headerShown: false }}>
        <Tabs.Screen name="index" options={{ title: "Assistant" }} />
        <Tabs.Screen name="extensions" options={{ title: "Extensions" }} />
        <Tabs.Screen name="settings" options={{ title: "Settings" }} />
      </Tabs>
    </AppConfigProvider>
  );
}

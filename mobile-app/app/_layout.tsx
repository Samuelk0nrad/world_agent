import { Tabs } from "expo-router";
import { AppConfigProvider } from "../src/state/app-config";
import { colors, spacing, typography } from "../src/theme";

export default function RootLayout() {
  return (
    <AppConfigProvider>
      <Tabs
        screenOptions={{
          headerShown: false,
          tabBarActiveTintColor: colors.accent.subtle,
          tabBarInactiveTintColor: colors.text.muted,
          tabBarStyle: {
            backgroundColor: colors.surface.elevated,
            borderTopColor: colors.border.subtle,
            paddingTop: spacing.xs,
          },
          tabBarLabelStyle: {
            fontSize: typography.sizes.xs,
            fontWeight: typography.weights.semibold,
          },
          sceneStyle: {
            backgroundColor: colors.background.default,
          },
        }}
      >
        <Tabs.Screen name="index" options={{ title: "Assistant" }} />
        <Tabs.Screen name="extensions" options={{ title: "Extensions" }} />
        <Tabs.Screen name="settings" options={{ title: "Settings" }} />
      </Tabs>
    </AppConfigProvider>
  );
}

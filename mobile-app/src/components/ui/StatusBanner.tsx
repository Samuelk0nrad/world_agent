import { StyleSheet, Text, View } from "react-native";
import { colors, radius, spacing, typography } from "../../theme";

type StatusTone = "info" | "success" | "warning" | "error";

type StatusBannerProps = {
  message: string;
  tone?: StatusTone;
  announce?: boolean;
};

const toneStyles = {
  info: {
    borderColor: colors.accent.subtle,
    textColor: colors.accent.subtle,
    backgroundColor: "rgba(56, 189, 248, 0.12)",
  },
  success: {
    borderColor: colors.status.success,
    textColor: colors.status.success,
    backgroundColor: "rgba(16, 185, 129, 0.12)",
  },
  warning: {
    borderColor: colors.status.warning,
    textColor: colors.status.warning,
    backgroundColor: "rgba(245, 158, 11, 0.12)",
  },
  error: {
    borderColor: colors.status.error,
    textColor: colors.status.error,
    backgroundColor: "rgba(239, 68, 68, 0.12)",
  },
} as const;

export function StatusBanner({ message, tone = "info", announce = false }: StatusBannerProps) {
  const palette = toneStyles[tone];
  return (
    <View
      style={[styles.container, { borderColor: palette.borderColor, backgroundColor: palette.backgroundColor }]}
      accessibilityLabel={`${tone} status`}
      accessibilityRole={announce ? "alert" : "text"}
      accessibilityLiveRegion={announce ? "polite" : "none"}
    >
      <Text style={[styles.text, { color: palette.textColor }]}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderRadius: radius.sm,
    borderWidth: 1,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    minHeight: 44,
    justifyContent: "center",
  },
  text: {
    fontSize: typography.sizes.xs,
  },
});

import type { TextStyle } from "react-native";

type FontWeight = NonNullable<TextStyle["fontWeight"]>;

export const colors = {
  background: {
    default: "#0b1020",
  },
  surface: {
    default: "#111827",
    elevated: "#0f172a",
  },
  text: {
    primary: "#f8fafc",
    secondary: "#cbd5e1",
    muted: "#94a3b8",
    inverse: "#ffffff",
  },
  accent: {
    primary: "#2563eb",
    secondary: "#0f766e",
    subtle: "#93c5fd",
  },
  status: {
    info: "#38bdf8",
    success: "#10b981",
    warning: "#f59e0b",
    error: "#ef4444",
  },
  border: {
    subtle: "#334155",
    strong: "#475569",
  },
} as const;

export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  xxl: 24,
} as const;

export const radius = {
  sm: 8,
  md: 10,
  lg: 12,
  xl: 16,
  pill: 999,
} as const;

export const typography = {
  sizes: {
    xs: 12,
    sm: 14,
    md: 16,
    lg: 20,
    xl: 24,
    xxl: 28,
  },
  weights: {
    regular: "400" as FontWeight,
    medium: "500" as FontWeight,
    semibold: "600" as FontWeight,
    bold: "700" as FontWeight,
  },
  lineHeights: {
    compact: 16,
    body: 20,
    relaxed: 24,
  },
} as const;

export const textVariants = {
  screenTitle: {
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
    color: colors.text.primary,
  },
  screenSubtitle: {
    fontSize: typography.sizes.sm,
    lineHeight: typography.lineHeights.body,
    color: colors.text.secondary,
  },
  body: {
    fontSize: typography.sizes.sm,
    lineHeight: typography.lineHeights.body,
    color: colors.text.secondary,
  },
  caption: {
    fontSize: typography.sizes.xs,
    color: colors.text.muted,
  },
} as const satisfies Record<string, TextStyle>;

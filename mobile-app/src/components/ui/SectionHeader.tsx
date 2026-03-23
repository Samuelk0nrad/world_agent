import { StyleSheet, Text, View } from "react-native";
import { colors, spacing, textVariants, typography } from "../../theme";

type SectionHeaderProps = {
  title: string;
  subtitle?: string;
};

export function SectionHeader({ title, subtitle }: SectionHeaderProps) {
  return (
    <View style={styles.container}>
      <Text style={styles.title} accessibilityRole="header">
        {title}
      </Text>
      {subtitle ? <Text style={styles.subtitle}>{subtitle}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    gap: spacing.xs,
  },
  title: {
    ...textVariants.screenTitle,
    fontSize: typography.sizes.xxl,
  },
  subtitle: {
    ...textVariants.screenSubtitle,
  },
});

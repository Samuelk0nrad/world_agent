import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Clipboard,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { runAgent } from "../src/api/client";
import type { ChatMessage } from "../src/state/chat-transcript";
import { loadChatTranscript, persistChatTranscript } from "../src/state/chat-transcript";
import { Card, PrimaryButton, Screen, SectionHeader, StatusBanner } from "../src/components/ui";
import { useAppConfig } from "../src/state/app-config";
import { colors, radius, spacing, textVariants, typography } from "../src/theme";

type StatusTone = "info" | "success" | "warning" | "error";

type AssistantStatus = {
  message: string;
  tone: StatusTone;
};

const INITIAL_ASSISTANT_MESSAGE =
  "Hi, I am ready. Ask me to search the web, manage email, or remember context.";

const DEFAULT_MESSAGES: ChatMessage[] = [
  {
    role: "assistant",
    text: INITIAL_ASSISTANT_MESSAGE,
  },
];

function findLastUserPrompt(messages: ChatMessage[]): string {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const item = messages[index];
    if (item.role === "user") {
      return item.text;
    }
  }

  return "";
}

export default function AssistantScreen() {
  const { backendUrl, googleAccessToken, sessionId } = useAppConfig();
  const [message, setMessage] = useState("");
  const [status, setStatus] = useState<AssistantStatus>({
    message: "Ready for your next instruction.",
    tone: "info",
  });
  const [messages, setMessages] = useState<ChatMessage[]>(DEFAULT_MESSAGES);
  const [isRunning, setIsRunning] = useState(false);
  const [isHydrating, setIsHydrating] = useState(true);

  const lastUserPrompt = useMemo(() => findLastUserPrompt(messages), [messages]);

  useEffect(() => {
    let isCancelled = false;

    setIsHydrating(true);
    void (async () => {
      const restored = await loadChatTranscript(sessionId);
      if (isCancelled) {
        return;
      }

      if (restored.messages.length > 0) {
        setMessages(restored.messages);
        if (restored.warning) {
          setStatus({ message: restored.warning, tone: "warning" });
        } else {
          setStatus({ message: "Restored local conversation history.", tone: "info" });
        }
      } else {
        setMessages(DEFAULT_MESSAGES);
        if (restored.warning) {
          setStatus({ message: restored.warning, tone: "warning" });
        } else {
          setStatus({ message: "Ready for your next instruction.", tone: "info" });
        }
      }

      setIsHydrating(false);
    })();

    return () => {
      isCancelled = true;
    };
  }, [sessionId]);

  useEffect(() => {
    if (isHydrating) {
      return;
    }

    let isCancelled = false;
    void (async () => {
      const persistenceError = await persistChatTranscript(sessionId, messages);
      if (isCancelled || !persistenceError) {
        return;
      }

      setStatus({
        message: persistenceError,
        tone: "warning",
      });
    })();

    return () => {
      isCancelled = true;
    };
  }, [isHydrating, messages, sessionId]);

  const runPrompt = useCallback(
    async (input: string, source: "send" | "retry") => {
      const text = input.trim();
      if (!text) {
        setStatus({ message: "Type a message first.", tone: "warning" });
        return;
      }

      setMessages((prev) => [...prev, { role: "user", text }]);
      setIsRunning(true);
      setStatus({
        message: source === "retry" ? "Retrying your previous prompt..." : "Assistant is running the agent loop...",
        tone: "info",
      });

      try {
        const result = await runAgent(text, sessionId, { backendUrl, googleAccessToken });
        setMessages((prev) => [...prev, { role: "assistant", text: result.reply }]);
        setStatus({
          message: `Agent loop completed in ${result.steps.length} step(s).`,
          tone: "success",
        });
      } catch (error) {
        setStatus({
          message: error instanceof Error ? error.message : "Failed to send.",
          tone: "error",
        });
      } finally {
        setIsRunning(false);
      }
    },
    [backendUrl, googleAccessToken, sessionId],
  );

  const onSend = useCallback(() => {
    if (isRunning || isHydrating) {
      return;
    }

    const text = message.trim();
    if (!text) {
      setStatus({ message: "Type a message first.", tone: "warning" });
      return;
    }

    setMessage("");
    void runPrompt(text, "send");
  }, [isHydrating, isRunning, message, runPrompt]);

  const onRetryLastPrompt = useCallback(() => {
    if (isRunning || isHydrating) {
      return;
    }

    if (!lastUserPrompt) {
      setStatus({ message: "No previous user prompt available to retry.", tone: "warning" });
      return;
    }

    void runPrompt(lastUserPrompt, "retry");
  }, [isHydrating, isRunning, lastUserPrompt, runPrompt]);

  const onClearConversation = useCallback(() => {
    if (isRunning || isHydrating) {
      return;
    }

    Alert.alert("Clear conversation?", "This removes locally saved chat history for this session.", [
      {
        text: "Cancel",
        style: "cancel",
      },
      {
        text: "Clear",
        style: "destructive",
        onPress: () => {
          setMessages(DEFAULT_MESSAGES);
          setMessage("");
          setStatus({ message: "Conversation cleared.", tone: "success" });
        },
      },
    ]);
  }, [isHydrating, isRunning]);

  const onCopyAssistantResponse = useCallback((text: string) => {
    const content = text.trim();
    if (!content) {
      setStatus({ message: "Nothing to copy from this response.", tone: "warning" });
      return;
    }

    try {
      Clipboard.setString(content);
      setStatus({ message: "Assistant response copied to clipboard.", tone: "success" });
    } catch (error) {
      setStatus({
        message: error instanceof Error ? `Copy failed: ${error.message}` : "Copy failed.",
        tone: "error",
      });
    }
  }, []);

  return (
    <Screen style={styles.screen}>
      <Card style={styles.headerCard}>
        <SectionHeader
          title="WorldAgent Assistant"
          subtitle="Delegate web and email tasks to the backend agent loop."
        />
        <View style={styles.summaryGrid}>
          <View style={styles.summaryItem}>
            <Text style={styles.summaryLabel}>Session</Text>
            <Text style={styles.summaryValue} numberOfLines={1}>
              {sessionId || "Auto-generated"}
            </Text>
          </View>
          <View style={styles.summaryItem}>
            <Text style={styles.summaryLabel}>Messages</Text>
            <Text style={styles.summaryValue}>{messages.length}</Text>
          </View>
        </View>
        <Text style={styles.connectionValue} numberOfLines={1}>
          Backend: {backendUrl}
        </Text>
      </Card>

      <Card style={styles.conversationCard}>
        <View style={styles.conversationHeader}>
          <Text style={styles.conversationTitle}>Conversation</Text>
          <Text style={styles.conversationSubtitle}>
            {isHydrating ? "Restoring local history..." : isRunning ? "Assistant is responding..." : "Live session"}
          </Text>
        </View>
        <ScrollView style={styles.chatViewport} contentContainerStyle={styles.chatList}>
          {messages.map((item, index) => {
            const isUser = item.role === "user";

            return (
              <View key={`${item.role}-${index}`} style={[styles.messageRow, isUser && styles.userMessageRow]}>
                <Card style={[styles.bubble, isUser ? styles.userBubble : styles.assistantBubble]}>
                  <Text style={[styles.bubbleRole, isUser && styles.userBubbleText]}>
                    {isUser ? "You" : "Assistant"}
                  </Text>
                  <Text style={[styles.bubbleText, isUser && styles.userBubbleText]}>{item.text}</Text>
                  {!isUser ? (
                    <Pressable onPress={() => onCopyAssistantResponse(item.text)} style={styles.copyAction}>
                      <Text style={styles.copyActionText}>Copy response</Text>
                    </Pressable>
                  ) : null}
                </Card>
              </View>
            );
          })}
        </ScrollView>
      </Card>

      <Card style={styles.composerCard}>
        <View style={styles.composerHeader}>
          <Text style={styles.composerTitle}>Compose</Text>
          {isRunning ? (
            <View style={styles.loadingState}>
              <ActivityIndicator size="small" color={colors.status.info} />
              <Text style={styles.loadingText}>Running…</Text>
            </View>
          ) : null}
        </View>

        <View style={styles.quickActionRow}>
          <PrimaryButton
            title="Retry last prompt"
            onPress={onRetryLastPrompt}
            disabled={isRunning || isHydrating || !lastUserPrompt}
            variant="secondary"
            style={styles.quickActionButton}
          />
          <PrimaryButton
            title="Clear conversation"
            onPress={onClearConversation}
            disabled={isRunning || isHydrating}
            variant="secondary"
            style={styles.quickActionButton}
          />
        </View>

        <TextInput
          style={styles.input}
          placeholder="Ask the assistant to do something..."
          placeholderTextColor={colors.text.muted}
          value={message}
          onChangeText={setMessage}
          multiline
          textAlignVertical="top"
          editable={!isRunning && !isHydrating}
        />
        <PrimaryButton
          title={isRunning ? "Running agent loop..." : "Send to Assistant"}
          onPress={onSend}
          disabled={isRunning || isHydrating}
          style={styles.sendButton}
        />
        <Text style={styles.composerHint}>
          {isHydrating
            ? "Restoring your local conversation..."
            : isRunning
              ? "Please wait while the assistant finishes this request."
              : "Requests are sent to the configured backend."}
        </Text>
      </Card>

      <StatusBanner message={status.message} tone={status.tone} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg,
  },
  headerCard: {
    gap: spacing.md,
  },
  summaryGrid: {
    flexDirection: "row",
    gap: spacing.sm,
  },
  summaryItem: {
    flex: 1,
    borderWidth: 1,
    borderColor: colors.border.subtle,
    borderRadius: radius.sm,
    backgroundColor: colors.surface.elevated,
    padding: spacing.sm,
    gap: spacing.xs,
  },
  summaryLabel: {
    color: colors.accent.subtle,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  summaryValue: {
    color: colors.text.primary,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.medium,
  },
  connectionValue: {
    ...textVariants.caption,
    color: colors.text.secondary,
  },
  conversationCard: {
    flex: 1,
    gap: spacing.md,
  },
  conversationHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: spacing.sm,
  },
  conversationTitle: {
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
    color: colors.text.primary,
  },
  conversationSubtitle: {
    ...textVariants.caption,
  },
  chatViewport: {
    flex: 1,
  },
  chatList: {
    gap: spacing.md,
    paddingBottom: spacing.sm,
  },
  messageRow: {
    alignItems: "flex-start",
  },
  userMessageRow: {
    alignItems: "flex-end",
  },
  bubble: {
    gap: spacing.xs,
    maxWidth: "92%",
  },
  assistantBubble: {
    borderColor: colors.border.strong,
    backgroundColor: colors.surface.elevated,
  },
  userBubble: {
    borderColor: colors.accent.primary,
    backgroundColor: colors.accent.primary,
  },
  bubbleRole: {
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    color: colors.accent.subtle,
  },
  bubbleText: {
    ...textVariants.body,
    color: colors.text.primary,
  },
  userBubbleText: {
    color: colors.text.inverse,
  },
  copyAction: {
    marginTop: spacing.xs,
    alignSelf: "flex-start",
    borderWidth: 1,
    borderColor: colors.border.strong,
    borderRadius: radius.pill,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
  },
  copyActionText: {
    ...textVariants.caption,
    color: colors.accent.subtle,
    fontWeight: typography.weights.semibold,
  },
  composerCard: {
    gap: spacing.sm,
  },
  composerHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: spacing.sm,
  },
  composerTitle: {
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.semibold,
    color: colors.text.primary,
  },
  loadingState: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
  loadingText: {
    ...textVariants.caption,
    color: colors.status.info,
  },
  quickActionRow: {
    flexDirection: "row",
    gap: spacing.sm,
  },
  quickActionButton: {
    flex: 1,
  },
  input: {
    minHeight: 116,
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: colors.border.strong,
    backgroundColor: colors.surface.elevated,
    color: colors.text.primary,
    padding: spacing.md,
  },
  sendButton: {
    marginTop: spacing.xs,
  },
  composerHint: {
    ...textVariants.caption,
    color: colors.text.secondary,
  },
});

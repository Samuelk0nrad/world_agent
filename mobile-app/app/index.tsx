import { useState } from "react";
import { Pressable, SafeAreaView, ScrollView, StyleSheet, Text, TextInput, View } from "react-native";
import { runAgent } from "../src/api/client";
import { useAppConfig } from "../src/state/app-config";

type ChatMessage = {
  role: "user" | "assistant";
  text: string;
};

export default function AssistantScreen() {
  const { backendUrl, googleAccessToken } = useAppConfig();
  const [message, setMessage] = useState("");
  const [status, setStatus] = useState("Ready");
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: "assistant",
      text: "Hi, I am ready. Ask me to search the web, manage email, or remember context.",
    },
  ]);
  const [isRunning, setIsRunning] = useState(false);

  const onSend = async () => {
    const text = message.trim();
    if (!text) {
      setStatus("Type a message first.");
      return;
    }

    setMessages((prev) => [...prev, { role: "user", text }]);
    setMessage("");
    setIsRunning(true);

    try {
      const result = await runAgent(text, 4, { backendUrl, googleAccessToken });
      setMessages((prev) => [...prev, { role: "assistant", text: result.reply }]);
      setStatus(`Agent loop completed in ${result.steps.length} step(s).`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Failed to send.");
    } finally {
      setIsRunning(false);
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <Text style={styles.title}>WorldAgent Assistant</Text>
      <Text style={styles.subtitle}>Chat-first interface powered by the backend agent loop.</Text>
      <View style={styles.connectionCard}>
        <Text style={styles.connectionLabel}>Backend</Text>
        <Text style={styles.connectionValue}>{backendUrl}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.chatList}>
        {messages.map((item, index) => (
          <View
            key={`${item.role}-${index}`}
            style={[styles.bubble, item.role === "user" ? styles.userBubble : styles.assistantBubble]}
          >
            <Text style={styles.bubbleRole}>{item.role === "user" ? "You" : "Assistant"}</Text>
            <Text style={styles.bubbleText}>{item.text}</Text>
          </View>
        ))}
      </ScrollView>

      <View style={styles.card}>
        <TextInput
          style={styles.input}
          placeholder="Ask the assistant to do something..."
          value={message}
          onChangeText={setMessage}
          multiline
        />
        <Pressable style={[styles.button, isRunning && styles.buttonDisabled]} onPress={onSend} disabled={isRunning}>
          <Text style={styles.buttonText}>{isRunning ? "Running loop..." : "Send"}</Text>
        </Pressable>
      </View>

      <Text style={styles.status}>{status}</Text>
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
    fontSize: 28,
    fontWeight: "700",
  },
  subtitle: {
    color: "#cbd5e1",
    fontSize: 14,
  },
  card: {
    backgroundColor: "#111827",
    borderRadius: 12,
    padding: 12,
    gap: 10,
  },
  connectionCard: {
    backgroundColor: "#111827",
    borderRadius: 10,
    borderWidth: 1,
    borderColor: "#334155",
    padding: 10,
    gap: 2,
  },
  connectionLabel: {
    color: "#93c5fd",
    fontSize: 12,
    fontWeight: "600",
  },
  connectionValue: {
    color: "#f8fafc",
    fontSize: 12,
  },
  chatList: {
    gap: 8,
  },
  bubble: {
    borderRadius: 10,
    borderWidth: 1,
    padding: 10,
    gap: 4,
  },
  userBubble: {
    borderColor: "#2563eb",
    backgroundColor: "#1d4ed8",
  },
  assistantBubble: {
    borderColor: "#334155",
    backgroundColor: "#111827",
  },
  bubbleRole: {
    color: "#93c5fd",
    fontSize: 12,
    fontWeight: "600",
  },
  bubbleText: {
    color: "#f8fafc",
    fontSize: 14,
  },
  input: {
    minHeight: 120,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#334155",
    color: "#f8fafc",
    padding: 10,
    textAlignVertical: "top",
  },
  button: {
    backgroundColor: "#2563eb",
    borderRadius: 8,
    paddingVertical: 10,
    alignItems: "center",
  },
  buttonDisabled: {
    opacity: 0.7,
  },
  buttonText: {
    color: "#ffffff",
    fontWeight: "600",
  },
  status: {
    color: "#93c5fd",
    fontSize: 13,
  },
});

When a tool is required, respond with exactly one JSON object:
{"id":"<tool-name>","type":"function","arguments":{...}}

Rules:
- Do not include prose, markdown, or code fences.
- "id" must match a tool listed in <tools>.
- "type" must be "function".
- "arguments" must be a JSON object that matches the tool signature schema.

If no tool is needed, answer normally.

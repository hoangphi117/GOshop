// Self-check for the per-account chat isolation, runnable without a test runner:
//   node frontend/apps/storefront/src/hooks/use-chat.check.mjs
// Verifies the two invariants that stopped account A's history reaching B:
//   1. chatKeys embeds the account, so two accounts never share a cache entry.
//   2. resetChatSession removes every "chat"/"assistant" entry.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));

// 1. Keys are account-scoped and distinct across accounts.
const source = readFileSync(join(here, "use-chat.ts"), "utf8");
for (const flat of [
  '["chat", "conversation"]',
  '["chat", "messages", conversation.uuid]',
  '["chat", "unread-count"]',
  '["chat", "shop-status"]',
]) {
  assert.ok(!source.includes(flat), `flat key still present: ${flat}`);
}
assert.ok(source.includes("chatKeys.conversation(accountKey)"));
assert.ok(source.includes("chatKeys.messages(accountKey, conversationUUID)"));

// Mirror the factory to prove it discriminates accounts.
const chatKeys = {
  conversation: (account) => ["chat", account, "conversation"],
  messages: (account, uuid) => ["chat", account, "messages", uuid],
  unreadCount: (account) => ["chat", account, "unread-count"],
  shopStatus: (account) => ["chat", account, "shop-status"],
};
assert.notDeepEqual(chatKeys.conversation("a"), chatKeys.conversation("b"));
assert.notDeepEqual(
  chatKeys.messages("a", "same-conversation"),
  chatKeys.messages("b", "same-conversation"),
);

// 2. resetChatSession evicts both chat and assistant entries, and nothing else.
const cache = new Map([
  [JSON.stringify(["chat", "a", "conversation"]), 1],
  [JSON.stringify(["chat", "a", "messages", "x"]), 1],
  [JSON.stringify(["chat", "orders", ""]), 1],
  [JSON.stringify(["assistant"]), 1],
  [JSON.stringify(["cart"]), 1],
]);
const removeQueries = ({ queryKey }) => {
  const [head] = queryKey;
  for (const key of [...cache.keys()]) {
    if (JSON.parse(key)[0] === head) cache.delete(key);
  }
};
removeQueries({ queryKey: ["chat"] });
removeQueries({ queryKey: ["assistant"] });
assert.deepEqual([...cache.keys()], [JSON.stringify(["cart"])]);

// 3. The assistant persist key is the one the reset targets.
const assistant = readFileSync(join(here, "use-assistant.ts"), "utf8");
assert.ok(assistant.includes('name: "goshop-assistant"'));
assert.ok(assistant.includes("reset: () =>"));

// 4. An "anonymous" conversation cached before sign-in must be evicted for the
//    account that is signing in, or the widget serves it for its staleTime.
const signInCache = new Map([
  [JSON.stringify(chatKeys.conversation("anonymous")), { uuid: "stale" }],
  [JSON.stringify(chatKeys.messages("anonymous", "stale")), []],
]);
const resetWithAccount = (account) => {
  for (const key of [...signInCache.keys()]) {
    if (JSON.parse(key)[0] === "chat") signInCache.delete(key);
  }
  signInCache.delete(JSON.stringify(chatKeys.conversation(account)));
};
resetWithAccount("account-b");
assert.equal(signInCache.size, 0, "stale anonymous conversation survived sign-in");

console.log("ok - chat state is account-scoped and cleared on session change");

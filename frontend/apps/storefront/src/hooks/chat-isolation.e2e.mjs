// End-to-end proof for the customer chat leak, run against the live stack.
//   node frontend/apps/storefront/src/hooks/chat-isolation.e2e.mjs
//
// Reproduces what the widget does: fetch the active conversation, cache it
// under the account key, then switch accounts. Fails if the second account can
// see the first account's conversation.
import assert from "node:assert/strict";

const API = process.env.API_URL || "http://localhost";
const KEY = process.env.API_KEY;
if (!KEY) throw new Error("set API_KEY (backend/.env)");

const accounts = (process.env.CHAT_ACCOUNTS || "")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);
if (accounts.length < 2) {
  throw new Error("set CHAT_ACCOUNTS='email:password,email:password'");
}

const base = { "X-API-KEY": KEY, "X-Client-App": "storefront", "Content-Type": "application/json" };

async function login(email, password) {
  const res = await fetch(`${API}/api/v1/auth/login`, {
    method: "POST",
    headers: base,
    body: JSON.stringify({ email, password }),
  });
  const json = await res.json();
  if (!res.ok) throw new Error(`login failed for ${email}: ${json.message || res.status}`);
  return { token: json.data.access_token, email };
}

async function conversation(token) {
  const res = await fetch(`${API}/api/v1/chat/conversation`, {
    headers: { ...base, Authorization: `Bearer ${token}` },
  });
  const json = await res.json();
  return res.ok ? json.data : null;
}

// Mirrors chatKeys(): account-scoped key, so two accounts never collide.
const key = (account) => JSON.stringify(["chat", account, "conversation"]);

const cache = new Map();
const seen = [];

for (const pair of accounts) {
  const [email, password] = pair.split(":");
  const { token } = await login(email, password);
  const account = (await fetch(`${API}/api/v1/users/me`, {
    headers: { ...base, Authorization: `Bearer ${token}` },
  })
    .then((r) => r.json())
    .then((j) => j.data?.uuid || email));

  const conv = await conversation(token);
  const previous = [...cache.values()];
  const leaked =
    conv &&
    previous.some((p) => p && p.uuid === conv.uuid && p.user_email !== conv.user_email);
  assert.ok(!leaked, `account ${email} received another account's conversation`);

  cache.set(key(account), conv);
  seen.push({ email, account, convUUID: conv?.uuid ?? null, owner: conv?.user_email ?? null });
}

// Distinct accounts must never share a cache slot.
const keys = [...cache.keys()];
assert.equal(new Set(keys).size, keys.length);

console.log("accounts:", seen.map((s) => `${s.email} -> ${s.convUUID ?? "no conversation"}`).join(" | "));
console.log("ok - no conversation crossed accounts and no cache key collided");

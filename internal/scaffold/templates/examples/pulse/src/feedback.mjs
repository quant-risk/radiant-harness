// Collect feedback — minimal implementation (golden example).
// Covers AC-1 (accept valid), AC-2 (reject empty), AC-3 (reject oversized).

const MAX = 1000;
const store = [];

export function submitFeedback({ text, context } = {}) {
  if (!text || !text.trim()) return { error: "empty text" };          // AC-2
  if (text.length > MAX) return { error: "text too long" };           // AC-3
  const id = String(store.length + 1);
  store.push({ id, text, context });                                   // AC-1
  return { id };
}

export const _store = store;

// Safe localStorage wrapper. Every key is namespaced `bip:v1:`; every access is
// try/catch-guarded so the app works with storage disabled (private mode, quota, blocked).
const PREFIX = 'bip:v1:'

export const keys = {
  lang: () => `${PREFIX}lang`,
  theme: () => `${PREFIX}theme`,
  draft: (userId, assessmentId) => `${PREFIX}draft:${userId}:${assessmentId}`,
  draftPrefix: () => `${PREFIX}draft:`,
  wizard: (userId) => `${PREFIX}wizard:${userId}`,
}

export function getItem(key) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

export function setItem(key, value) {
  try {
    localStorage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

export function removeItem(key) {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}

export function getJSON(key, fallback = null) {
  const raw = getItem(key)
  if (raw == null) return fallback
  try {
    return JSON.parse(raw)
  } catch {
    return fallback
  }
}

export function setJSON(key, value) {
  try {
    return setItem(key, JSON.stringify(value))
  } catch {
    return false
  }
}

export function keysWithPrefix(prefix) {
  const out = []
  try {
    for (let i = 0; i < localStorage.length; i += 1) {
      const k = localStorage.key(i)
      if (k && k.startsWith(prefix)) out.push(k)
    }
  } catch {
    /* ignore */
  }
  return out
}

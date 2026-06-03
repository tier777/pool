const AUTH_KEY = "pool_token";

let timeout = null;
let isTyping = false;
let pollInterval = null;
let lastServerAt = 0;

function getToken() {
  return sessionStorage.getItem(AUTH_KEY);
}

function setToken(token) {
  sessionStorage.setItem(AUTH_KEY, token);
}

function clearToken() {
  sessionStorage.removeItem(AUTH_KEY);
}

function authHeaders(extra = {}) {
  const headers = { ...extra };
  const token = getToken();
  if (token) {
    headers.Authorization = "Bearer " + token;
  }
  return headers;
}

function showAuthGate() {
  document.getElementById("auth-gate").classList.remove("hidden");
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

function hideAuthGate() {
  document.getElementById("auth-gate").classList.add("hidden");
}

async function apiFetch(url, options = {}) {
  const res = await fetch(url, {
    ...options,
    headers: authHeaders(options.headers),
  });

  if (res.status === 401) {
    clearToken();
    showAuthGate();
    throw new Error("unauthorized");
  }

  if (!res.ok) {
    throw new Error("request failed");
  }

  return res;
}

async function load() {
  if (isTyping) return;

  const res = await apiFetch("/api/pool");
  const data = await res.json();

  if (data.updated_at <= lastServerAt) {
    return;
  }

  const text = document.getElementById("text");
  if (text.value !== data.content) {
    text.value = data.content;
  }
  lastServerAt = data.updated_at;
}

async function save() {
  const content = document.getElementById("text").value;

  isTyping = true;
  showStatus("saving");

  try {
    const res = await apiFetch("/api/pool", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    });

    const data = await res.json();
    lastServerAt = data.updated_at;
    showStatus("saved");
  } catch (err) {
    if (err.message !== "unauthorized") {
      showStatus("error");
    }
  } finally {
    isTyping = false;
  }
}

function debounceSave() {
  clearTimeout(timeout);

  isTyping = true;
  showStatus("saving");

  timeout = setTimeout(save, 500);
}

const STATUS_LABELS = {
  saved: "[saved ✓]",
  saving: "[saving...]",
  error: "[error X]",
  copied: "[copied !]",
};

let statusTimer = null;
let statusSeq = 0;
let lastStatusKey = "saved";

function showStatus(key) {
  const el = document.getElementById("status");
  const label = STATUS_LABELS[key] ?? `[${key}]`;

  if (key === lastStatusKey && el.textContent === label) {
    return;
  }

  const prevKey = lastStatusKey;
  lastStatusKey = key;

  clearTimeout(statusTimer);
  const seq = ++statusSeq;

  const apply = () => {
    if (seq !== statusSeq) return;
    el.textContent = label;
    el.style.opacity = "1";
  };

  // saving → saved/error: swap without a second fade-out
  if (prevKey === "saving" && (key === "saved" || key === "error")) {
    apply();
    return;
  }

  el.style.opacity = "0";
  statusTimer = setTimeout(apply, 40);
}

async function copyText() {
  const content = document.getElementById("text").value;

  try {
    await navigator.clipboard.writeText(content);
  } catch {
    const text = document.getElementById("text");
    text.select();
    document.execCommand("copy");
  }

  showStatus("copied");
}

function clearText() {
  document.getElementById("text").value = "";
  save();
}

function startPolling() {
  if (pollInterval) return;
  pollInterval = setInterval(load, 2000);
}

async function unlock() {
  const password = document.getElementById("password").value;
  if (!password) return;

  setToken(password);

  try {
    await load();
    hideAuthGate();
    startPolling();
  } catch {
    clearToken();
    document.getElementById("password").value = "";
  }
}

async function init() {
  if (!getToken()) {
    showAuthGate();
    return;
  }

  try {
    await load();
    hideAuthGate();
    startPolling();
  } catch {
    showAuthGate();
  }
}

// events
document.addEventListener("touchstart", () => {}, true);
document.getElementById("text").addEventListener("input", debounceSave);
document.getElementById("copy").onclick = copyText;
document.getElementById("clear").onclick = clearText;
document.getElementById("unlock").onclick = unlock;
document.getElementById("password").addEventListener("keydown", (e) => {
  if (e.key === "Enter") unlock();
});

init();

const AUTH_KEY = "pool_token";

let timeout = null;
let isTyping = false;
let pollInterval = null;

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

  return res;
}

async function load() {
  if (isTyping) return;

  const res = await apiFetch("/api/pool");
  const data = await res.json();

  document.getElementById("text").value = data.content;
}

async function save() {
  const content = document.getElementById("text").value;

  await apiFetch("/api/pool", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });

  showStatus("saved");
}

function debounceSave() {
  clearTimeout(timeout);

  isTyping = true;
  showStatus("typing");

  timeout = setTimeout(() => {
    isTyping = false;
    save();
  }, 500);
}

function showStatus(text) {
  const el = document.getElementById("status");
  if (el.innerText.includes(text)) {
    return;
  }
  el.style.opacity = 0;
  setTimeout(() => {
    el.innerText = `[${text}]`;
    el.style.opacity = 1;
  }, 100);
}

function copyText() {
  const text = document.getElementById("text");
  text.select();
  document.execCommand("copy");
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

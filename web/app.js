let timeout = null;
let isTyping = false;
let pollInterval = null;
let lastServerAt = 0;
let csrfToken = "";

function showAuthGate(message = "") {
  const gate = document.getElementById("auth-gate");
  const main = document.querySelector("main");
  const password = document.getElementById("password");
  const error = document.getElementById("auth-error");

  gate.classList.remove("hidden");
  main.inert = true;
  main.setAttribute("aria-hidden", "true");
  error.textContent = message;
  error.hidden = !message;
  password.setAttribute("aria-invalid", String(Boolean(message)));
  requestAnimationFrame(() => password.focus());

  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

function hideAuthGate() {
  document.getElementById("auth-gate").classList.add("hidden");
  const main = document.querySelector("main");
  main.inert = false;
  main.removeAttribute("aria-hidden");
  document.getElementById("auth-error").hidden = true;
  document.getElementById("password").setAttribute("aria-invalid", "false");
  document.getElementById("text").focus();
}

async function apiFetch(url, options = {}) {
  const method = (options.method || "GET").toUpperCase();
  const headers = { ...options.headers };
  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }
  const res = await fetch(url, {
    ...options,
    credentials: "same-origin",
    headers,
  });

  if (res.status === 401) {
    csrfToken = "";
    showAuthGate("Password required.");
  }

  if (!res.ok) {
    const error = new Error("request failed");
    error.status = res.status;
    throw error;
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
    showStatus(err.status === 413 ? "tooLarge" : err.status === 429 ? "locked" : "error");
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
  loading: "[loading…]",
  saved: "[saved ✓]",
  saving: "[saving…]",
  error: "[error X]",
  locked: "[try later]",
  tooLarge: "[too large]",
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
  pollInterval = setInterval(() => {
    load().catch((err) => {
      if (err.status !== 401) {
        showStatus(err.status === 429 ? "locked" : "error");
      }
    });
  }, 2000);
}

async function unlock() {
  const input = document.getElementById("password");
  const button = document.getElementById("unlock");
  const password = input.value;
  if (!password || button.disabled) return;

  button.disabled = true;

  try {
    const res = await fetch("/api/login", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });
    input.value = "";
    if (!res.ok) {
      const error = new Error("login failed");
      error.status = res.status;
      throw error;
    }
    csrfToken = (await res.json()).csrf_token;
    await load();
    hideAuthGate();
    showStatus("saved");
    startPolling();
  } catch (err) {
    csrfToken = "";
    input.value = "";
    const message =
      err.status === 401
        ? "Wrong password. Try again."
        : err.status === 429
          ? "Too many attempts. Try again later."
          : "Cannot reach Pool. Check the connection and retry.";
    showAuthGate(message);
  } finally {
    button.disabled = false;
  }
}

async function init() {
  try {
    const res = await fetch("/api/session", { credentials: "same-origin" });
    if (!res.ok) {
      const error = new Error("session unavailable");
      error.status = res.status;
      throw error;
    }
    csrfToken = (await res.json()).csrf_token;
    await load();
    hideAuthGate();
    showStatus("saved");
    startPolling();
  } catch (err) {
    if (err.status !== 401) {
      showAuthGate(
        err.status === 429
          ? "Too many attempts. Try again later."
          : "Cannot reach Pool. Check the connection and retry.",
      );
    }
  }
}

async function logout() {
  const button = document.getElementById("logout");
  if (button.disabled) return;
  button.disabled = true;
  try {
    await apiFetch("/api/logout", { method: "POST" });
    csrfToken = "";
    lastServerAt = 0;
    document.getElementById("text").value = "";
    showAuthGate();
  } catch (err) {
    if (err.status !== 401) showStatus("error");
  } finally {
    button.disabled = false;
  }
}

// events
document.getElementById("text").addEventListener("input", debounceSave);
document.getElementById("copy").onclick = copyText;
document.getElementById("clear").onclick = clearText;
document.getElementById("logout").onclick = logout;
document.getElementById("unlock").onclick = unlock;
document.getElementById("password").addEventListener("keydown", (e) => {
  if (e.key === "Enter") unlock();
});
document.getElementById("auth-gate").addEventListener("keydown", (e) => {
  if (e.key !== "Tab") return;

  const password = document.getElementById("password");
  const unlock = document.getElementById("unlock");
  if (e.shiftKey && document.activeElement === password) {
    e.preventDefault();
    unlock.focus();
  } else if (!e.shiftKey && document.activeElement === unlock) {
    e.preventDefault();
    password.focus();
  }
});

init();

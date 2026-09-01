let timeout = null;
let isTyping = false;
let pollInterval = null;
let lastServerAt = 0;
let csrfToken = "";
let dragDepth = 0;

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
  if (pollInterval) clearInterval(pollInterval);
  pollInterval = null;
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

function showGlobalError(message = "") {
  const error = document.getElementById("global-error");
  error.textContent = message ? `[${message}]` : "";
  error.hidden = !message;
}

function loginError(status) {
  if (status === 401) return "Wrong password. Try again.";
  if (status === 429) return "Too many attempts. Try again later.";
  return status ? "Pool could not unlock. Try again." : "Cannot reach Pool. Check the connection and retry.";
}

async function apiFetch(url, options = {}) {
  const method = (options.method || "GET").toUpperCase();
  const headers = { ...options.headers };
  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }
  const res = await fetch(url, { ...options, credentials: "same-origin", headers });
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
  const data = await (await apiFetch("/api/pool")).json();
  if (data.updated_at <= lastServerAt) return;
  const text = document.getElementById("text");
  if (text.value !== data.content) text.value = data.content;
  lastServerAt = data.updated_at;
}

async function save() {
  isTyping = true;
  try {
    const res = await apiFetch("/api/pool", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content: document.getElementById("text").value }),
    });
    lastServerAt = (await res.json()).updated_at;
    showGlobalError();
  } catch (error) {
    showGlobalError(error.status === 413 ? "note too large" : error.status === 429 ? "try again later" : "note could not be saved");
  } finally {
    isTyping = false;
  }
}

function debounceSave() {
  clearTimeout(timeout);
  isTyping = true;
  timeout = setTimeout(save, 500);
}

async function copyText() {
  const text = document.getElementById("text");
  try {
    await navigator.clipboard.writeText(text.value);
  } catch {
    text.select();
    document.execCommand("copy");
  }
}

function clearText() {
  document.getElementById("text").value = "";
  save();
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function renderFiles(files) {
  const list = document.getElementById("files");
  list.replaceChildren();
  for (const file of files) {
    const row = document.createElement("article");
    row.className = "file-row";
    const details = document.createElement("div");
    details.className = "file-details";
    const name = document.createElement("strong");
    name.className = "file-name";
    name.textContent = file.name;
    const size = document.createElement("span");
    size.className = "file-size";
    size.textContent = formatBytes(file.size);
    details.append(name, size);

    const download = document.createElement("a");
    download.className = "file-action";
    download.href = `/api/files/${file.id}`;
    download.textContent = "↓";
    download.setAttribute("aria-label", `Download ${file.name}`);
    download.title = "Download";

    const remove = document.createElement("button");
    remove.className = "file-action file-delete";
    remove.type = "button";
    remove.textContent = "…";
    remove.setAttribute("aria-label", `Delete ${file.name}`);
    remove.title = "Delete";
    remove.onclick = () => deleteFile(file);
    row.append(details, download, remove);
    list.append(row);
  }
}

async function loadFiles() {
  renderFiles(await (await apiFetch("/api/files")).json());
}

async function uploadFiles(files) {
  const input = document.getElementById("file-input");
  const dropZone = document.getElementById("drop-zone");
  input.disabled = true;
  dropZone.setAttribute("aria-busy", "true");
  showGlobalError();
  try {
    for (const file of files) {
      const body = new FormData();
      body.append("file", file);
      await apiFetch("/api/files", { method: "POST", body });
    }
    await loadFiles();
  } catch (error) {
    showGlobalError("file upload failed — retry");
  } finally {
    input.value = "";
    input.disabled = false;
    dropZone.removeAttribute("aria-busy");
  }
}

async function deleteFile(file) {
  try {
    await apiFetch(`/api/files/${file.id}`, { method: "DELETE" });
    await loadFiles();
    showGlobalError();
  } catch {
    showGlobalError("file could not be deleted — retry");
  }
}

function startPolling() {
  if (pollInterval) return;
  pollInterval = setInterval(() => {
    Promise.all([load(), loadFiles()]).catch((error) => {
      if (error.status !== 401) showGlobalError(error.status === 429 ? "try again later" : "connection failed — retry");
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
    const res = await fetch("/api/login", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password }) });
    input.value = "";
    if (!res.ok) throw Object.assign(new Error("login failed"), { status: res.status });
    csrfToken = (await res.json()).csrf_token;
    await Promise.all([load(), loadFiles()]);
    hideAuthGate();
    showGlobalError();
    startPolling();
  } catch (error) {
    csrfToken = "";
    input.value = "";
    showAuthGate(loginError(error.status));
  } finally {
    button.disabled = false;
  }
}

async function init() {
  try {
    const res = await fetch("/api/session", { credentials: "same-origin" });
    if (!res.ok) throw Object.assign(new Error("session unavailable"), { status: res.status });
    csrfToken = (await res.json()).csrf_token;
    await Promise.all([load(), loadFiles()]);
    hideAuthGate();
    startPolling();
  } catch (error) {
    if (error.status !== 401) showAuthGate(error.status === 429 ? "Too many attempts. Try again later." : "Cannot reach Pool. Check the connection and retry.");
  }
}

document.getElementById("text").addEventListener("input", debounceSave);
document.getElementById("copy").onclick = copyText;
document.getElementById("clear").onclick = clearText;
document.getElementById("unlock").onclick = unlock;
document.getElementById("file-input").addEventListener("change", (event) => uploadFiles(event.target.files));
document.getElementById("password").addEventListener("keydown", (event) => { if (event.key === "Enter") unlock(); });
document.getElementById("auth-gate").addEventListener("keydown", (event) => {
  if (event.key !== "Tab") return;
  const password = document.getElementById("password");
  const unlockButton = document.getElementById("unlock");
  if (event.shiftKey && document.activeElement === password) { event.preventDefault(); unlockButton.focus(); }
  else if (!event.shiftKey && document.activeElement === unlockButton) { event.preventDefault(); password.focus(); }
});

document.addEventListener("dragenter", (event) => {
  if (!event.dataTransfer?.types.includes("Files")) return;
  event.preventDefault();
  dragDepth += 1;
  document.getElementById("drop-zone").classList.add("dragging");
});
document.addEventListener("dragover", (event) => {
  if (event.dataTransfer?.types.includes("Files")) event.preventDefault();
});
document.addEventListener("dragleave", (event) => {
  if (!event.dataTransfer?.types.includes("Files")) return;
  dragDepth = Math.max(0, dragDepth - 1);
  if (!dragDepth) document.getElementById("drop-zone").classList.remove("dragging");
});
document.addEventListener("drop", (event) => {
  if (!event.dataTransfer?.files.length) return;
  event.preventDefault();
  dragDepth = 0;
  document.getElementById("drop-zone").classList.remove("dragging");
  uploadFiles(event.dataTransfer.files);
});

init();

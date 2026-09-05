let timeout = null;
let isTyping = false;
let pollInterval = null;
let lastServerAt = 0;
let csrfToken = "";
let sessionRefresh = null;
let dragDepth = 0;
let statusTimeout = null;
let lastFiles = "";

function showAuthGate(message = "") {
  const gate = document.getElementById("auth-gate");
  const main = document.querySelector("main");
  const password = document.getElementById("password");
  const error = document.getElementById("auth-error");
  closeSettings(false);
  gate.classList.remove("hidden");
  main.inert = true;
  main.setAttribute("aria-hidden", "true");
  error.textContent = message ? `[${message}]` : "";
  if (message) showMessage(error);
  else hideMessage(error);
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
  hideMessage(document.getElementById("auth-error"));
  document.getElementById("password").setAttribute("aria-invalid", "false");
  document.getElementById("text").focus();
}

function showGlobalError(message = "") {
  const error = document.getElementById("global-error");
  clearTimeout(statusTimeout);
  error.classList.remove("status");
  error.textContent = message ? `[${message}]` : "";
  if (message) showMessage(error);
  else hideMessage(error);
}

function showGlobalStatus(message) {
  const error = document.getElementById("global-error");
  clearTimeout(statusTimeout);
  error.classList.add("status");
  error.textContent = `[${message}]`;
  showMessage(error);
  statusTimeout = setTimeout(() => hideMessage(error), 2000);
}

function showMessage(error) {
  error.hidden = false;
  requestAnimationFrame(() => error.classList.add("visible"));
}

function hideMessage(error) {
  error.classList.remove("visible");
  error.addEventListener("transitionend", () => {
    if (!error.classList.contains("visible")) {
      error.hidden = true;
      error.classList.remove("status");
    }
  }, { once: true });
}

function clearGlobalError() {
  const error = document.getElementById("global-error");
  if (!error.classList.contains("status")) showGlobalError();
}

function loginError(status) {
  if (status === 401) return "wrong password. try again";
  if (status === 429) return "too many attempts. try again later";
  return status ? "pool could not unlock. try again" : "cannot reach pool. check the connection and retry";
}

async function apiFetch(url, options = {}, retry = true) {
  const method = (options.method || "GET").toUpperCase();
  const headers = { ...options.headers };
  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }
  const res = await fetch(url, { ...options, credentials: "same-origin", headers });
  if (res.status === 401 && retry) {
    if (!sessionRefresh) {
      sessionRefresh = fetch("/api/session", { credentials: "same-origin" })
        .then(async session => {
          if (!session.ok) return false;
          csrfToken = (await session.json()).csrf_token;
          return true;
        }).finally(() => { sessionRefresh = null; });
    }
    if (await sessionRefresh) return apiFetch(url, options, false);
  }
  if (res.status === 401) {
    csrfToken = "";
    showAuthGate("password required");
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
    clearGlobalError();
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
    if (!document.execCommand("copy")) return;
  }
  showGlobalStatus("copied");
}

function clearText() {
  document.getElementById("text").value = "";
  showGlobalStatus("clear");
  save();
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} b`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} kb`;
  return `${(bytes / 1024 / 1024).toFixed(1)} mb`;
}

function renderFiles(files) {
  const list = document.getElementById("files");
  list.replaceChildren();
  for (const file of files) {
    const row = document.createElement("article");
    row.className = "file-row";
    const download = document.createElement("a");
    download.className = "file-download";
    download.href = `/api/files/${file.id}`;
    download.setAttribute("aria-label", `Download ${file.name}`);
    const details = document.createElement("div");
    details.className = "file-details";
    const name = document.createElement("strong");
    name.className = "file-name";
    name.textContent = file.name;
    const size = document.createElement("span");
    size.className = "file-size";
    size.textContent = formatBytes(file.size);
    details.append(name, size);
    download.append(details);

    const menu = document.createElement("div");
    menu.className = "file-menu";
    menu.hidden = true;
    menu.id = `file-menu-${file.id}`;
    menu.setAttribute("role", "menu");
    const remove = document.createElement("button");
    remove.type = "button";
    remove.textContent = "Delete";
    remove.setAttribute("role", "menuitem");
    remove.onclick = () => deleteFile(file);
    menu.append(remove);

    const trigger = document.createElement("button");
    trigger.className = "file-menu-trigger";
    trigger.type = "button";
    const icon = document.createElement("span");
    icon.className = "file-menu-icon";
    icon.setAttribute("aria-hidden", "true");
    icon.append(...Array.from({ length: 3 }, () => document.createElement("span")));
    trigger.append(icon);
    trigger.setAttribute("aria-label", `File actions for ${file.name}`);
    trigger.setAttribute("aria-haspopup", "menu");
    trigger.setAttribute("aria-controls", menu.id);
    trigger.setAttribute("aria-expanded", "false");
    trigger.onclick = () => {
      const isOpen = !menu.hidden;
      closeFileMenus();
      menu.hidden = isOpen;
      trigger.setAttribute("aria-expanded", String(!isOpen));
    };
    row.append(download, trigger, menu);
    list.append(row);
  }
}

function closeFileMenus() {
  document.querySelectorAll(".file-menu").forEach((menu) => { menu.hidden = true; });
  document.querySelectorAll(".file-menu-trigger").forEach((trigger) => { trigger.setAttribute("aria-expanded", "false"); });
}

async function loadFiles() {
  const files = await (await apiFetch("/api/files")).json();
  const filesState = JSON.stringify(files);
  if (filesState === lastFiles) return;
  lastFiles = filesState;
  renderFiles(files);
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
    if (error.status !== 401) showAuthGate(error.status === 429 ? "too many attempts. try again later" : "cannot reach pool. check the connection and retry");
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
document.addEventListener("click", (event) => {
  if (!event.target.closest(".file-row")) closeFileMenus();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeFileMenus();
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

let passwordEnabled = true;
let pendingPassword = null;
let settingsBusy = false;

function closeSettings(restoreFocus = true) {
  if (settingsBusy) return;
  document.getElementById("settings-screen").hidden = true;
  document.getElementById("pool-screen").hidden = false;
  document.getElementById("open-settings").hidden = false;
  document.getElementById("password-form").reset();
  pendingPassword = null;
  if (restoreFocus) document.getElementById("open-settings").focus();
}

function renderSettingsInput() {
  const enabled = document.getElementById("password-toggle").getAttribute("aria-checked") === "true";
  const input = document.getElementById("new-password");
  input.disabled = !enabled && pendingPassword === null;
  input.required = pendingPassword !== null || (enabled && !passwordEnabled);
  input.minLength = pendingPassword !== null ? 0 : 16;
  input.maxLength = 1024;
  input.autocomplete = pendingPassword !== null ? "current-password" : "new-password";
  input.setAttribute("aria-label", pendingPassword !== null ? "Current PIN" : "New PIN");
  document.getElementById("settings-message").textContent = pendingPassword !== null ? "Enter your current password to confirm." : !enabled && passwordEnabled ? "Without a PIN, anyone who can reach Pool can read and edit its contents and settings." : "";
}

async function openSettings() {
  closeFileMenus();
  const message = document.getElementById("settings-message");
  document.getElementById("pool-screen").hidden = true;
  document.getElementById("settings-screen").hidden = false;
  document.getElementById("open-settings").hidden = true;
  document.getElementById("password-form").reset();
  document.getElementById("password-error").hidden = true;
  pendingPassword = null;
  const controls = document.querySelectorAll("#settings-screen button, #settings-screen input");
  controls.forEach(control => { control.disabled = true; });
  message.textContent = "Loading…";
  try {
    const data = await (await apiFetch("/api/settings")).json();
    passwordEnabled = data.password_enabled;
    document.getElementById("password-toggle").setAttribute("aria-checked", String(passwordEnabled));
    controls.forEach(control => { control.disabled = false; });
    renderSettingsInput();
    if (!document.getElementById("settings-screen").hidden) document.getElementById("settings-screen").focus({preventScroll: true});
  } catch {
    message.textContent = "Could not load settings. Return to Pool and retry.";
  }
}

async function savePassword(event) {
  event.preventDefault();
  if (settingsBusy) return;
  const error = document.getElementById("password-error");
  const input = document.getElementById("new-password");
  const enabled = document.getElementById("password-toggle").getAttribute("aria-checked") === "true";
  error.hidden = true;
  if (pendingPassword === null) {
    if (enabled === passwordEnabled && (!enabled || !input.value)) {
      closeSettings();
      return;
    }
    pendingPassword = enabled ? input.value : "";
    input.value = "";
    if (passwordEnabled) {
      renderSettingsInput();
      input.focus();
      return;
    }
  }
  const body = JSON.stringify({password_enabled: enabled, current_password: passwordEnabled ? input.value : "", new_password: pendingPassword});
  settingsBusy = true;
  const controls = [...document.querySelectorAll("#settings-screen button, #settings-screen input")];
  controls.forEach(control => { control.disabled = true; });
  try {
    await apiFetch("/api/settings", {method: "POST", headers: {"Content-Type": "application/json"}, body});
    passwordEnabled = enabled;
    pendingPassword = null;
    input.value = "";
    renderSettingsInput();
    document.getElementById("settings-message").textContent = "Saved.";
  } catch (failure) {
    // Retry enabling from the new-password step; protected changes retain confirmation.
    if (!passwordEnabled) pendingPassword = null;
    error.textContent = failure.status === 403 ? "Current password is incorrect. Try again." : failure.status === 429 ? "Too many attempts. Try again in a minute." : failure.status === 400 ? "Use a password of at least 16 characters." : "Could not save settings. Try again.";
    error.hidden = false;
  } finally {
    settingsBusy = false;
    controls.forEach(control => { control.disabled = false; });
    // Keep success and error messages while restoring the field's enabled state.
    input.disabled = !enabled && pendingPassword === null;
    input.required = pendingPassword !== null || (enabled && !passwordEnabled);
    if (!document.getElementById("auth-gate").classList.contains("hidden")) closeSettings(false);
  }
}

document.getElementById("open-settings").onclick = openSettings;
document.getElementById("pool-home").onclick = event => { event.preventDefault(); closeSettings(); };
document.getElementById("password-toggle").onclick = () => {
  const toggle = document.getElementById("password-toggle");
  toggle.setAttribute("aria-checked", String(toggle.getAttribute("aria-checked") !== "true"));
  pendingPassword = null;
  document.getElementById("password-form").reset();
  document.getElementById("password-error").hidden = true;
  renderSettingsInput();
};
document.getElementById("password-form").onsubmit = savePassword;

init();

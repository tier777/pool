let timeout = null;
let isTyping = false;

async function load() {
  if (isTyping) return;

  const res = await fetch("/api/pool");
  const data = await res.json();

  document.getElementById("text").value = data.content;
}

async function save() {
  const content = document.getElementById("text").value;

  await fetch("/api/pool", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });

  showStatus("Saved");
}

function debounceSave() {
  clearTimeout(timeout);

  isTyping = true;
  showStatus("Typing...");

  timeout = setTimeout(() => {
    isTyping = false;
    save();
  }, 500);
}

function showStatus(text) {
  document.getElementById("status").innerText = text;
}

function copyText() {
  const text = document.getElementById("text");
  text.select();
  document.execCommand("copy");
  showStatus("Copied");
}

// events
document.getElementById("text").addEventListener("input", debounceSave);
document.getElementById("copy").onclick = copyText;

// polling
setInterval(load, 2000);

// initial load
load();

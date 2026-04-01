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

// events
document.getElementById("text").addEventListener("input", debounceSave);
document.getElementById("copy").onclick = copyText;

// polling
setInterval(load, 2000);

// initial load
load();

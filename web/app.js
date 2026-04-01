async function load() {
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
}

function copyText() {
  const text = document.getElementById("text");
  text.select();
  document.execCommand("copy");
}

document.getElementById("save").onclick = save;
document.getElementById("copy").onclick = copyText;

load();

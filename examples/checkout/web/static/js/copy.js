// data-copy="<id>" copies that element's text; an empty data-copy copies the enclosing code block.
document.addEventListener("click", async (e) => {
  const button = e.target.closest("[data-copy]");
  if (!button) return;
  const source = button.dataset.copy
    ? document.getElementById(button.dataset.copy)
    : button.closest(".codebox")?.querySelector("pre");
  try {
    await navigator.clipboard.writeText(source.textContent);
    button.textContent = "Copied";
  } catch {
    button.textContent = "Failed";
  }
  setTimeout(() => (button.textContent = "Copy"), 1400);
});

const isTyping = () => /INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName);

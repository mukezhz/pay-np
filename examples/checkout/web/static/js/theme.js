const themes = ["system", "light", "dark"];

function savedTheme() {
  try {
    return localStorage.getItem("theme") || "system";
  } catch {
    return "system";
  }
}

function applyTheme(theme) {
  if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
  else delete document.documentElement.dataset.theme;
  const button = document.querySelector(".theme-toggle");
  if (button) {
    button.setAttribute("aria-label", `Theme: ${theme}`);
    button.title = `Theme: ${theme} (click to change)`;
  }
}

applyTheme(savedTheme());
document.addEventListener("DOMContentLoaded", () => applyTheme(savedTheme()));

document.addEventListener("click", (e) => {
  if (!e.target.closest(".theme-toggle")) return;
  const next = themes[(themes.indexOf(savedTheme()) + 1) % themes.length];
  try {
    localStorage.setItem("theme", next);
  } catch {}
  applyTheme(next);
});

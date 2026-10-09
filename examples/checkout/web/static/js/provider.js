const verifyForm = document.getElementById("mobile-verify");
if (verifyForm) {
  const out = document.getElementById("mobile-verify-out");
  verifyForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const button = verifyForm.querySelector("button");
    button.disabled = true;
    try {
      const res = await fetch(verifyForm.action, {
        method: "POST",
        body: new URLSearchParams(new FormData(verifyForm)),
      });
      out.textContent = JSON.stringify(await res.json(), null, 2);
    } catch (err) {
      out.textContent = String(err);
    }
    out.hidden = false;
    button.disabled = false;
  });
}

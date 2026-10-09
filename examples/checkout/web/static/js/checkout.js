(() => {
  const form = document.getElementById("checkout"),
    amount = form.amount,
    btn = document.getElementById("pay-btn");
  const radios = [...form.querySelectorAll("input[name=provider]")];
  const fmt = (n) => "Rs " + n.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const sync = () => {
    const p = radios.find((r) => r.checked),
      n = Number(amount.value),
      ok = Number.isFinite(n) && n > 0;
    document.getElementById("sum-method").textContent = p ? p.dataset.label : "—";
    document.getElementById("sum-total").textContent = ok ? fmt(n) : "—";
    btn.firstChild.textContent = p ? `Pay ${ok ? fmt(n) + " " : ""}with ${p.dataset.label} ` : "Configure a provider ";
  };
  form.addEventListener("input", sync);
  form.addEventListener("change", sync);
  form.querySelectorAll("[data-amount]").forEach((b) =>
    b.addEventListener("click", () => {
      amount.value = b.dataset.amount;
      sync();
    }),
  );
  form.addEventListener("submit", () => {
    btn.disabled = true;
    btn.firstChild.textContent = "Redirecting… ";
  });
  addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      form.requestSubmit();
      return;
    }
    if (isTyping() && document.activeElement.type !== "radio") return;
    const r = radios[Number(e.key) - 1];
    if (r && !r.disabled) {
      r.checked = true;
      r.focus();
      sync();
    }
  });
  sync();
})();

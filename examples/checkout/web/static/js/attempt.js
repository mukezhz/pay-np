addEventListener("keydown", (e) => {
  if (e.key.toLowerCase() === "r" && !e.metaKey && !e.ctrlKey && !e.altKey && !isTyping()) {
    document.querySelector(".side-actions form").requestSubmit();
  }
});

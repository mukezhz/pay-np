addEventListener("keydown", (e) => {
  if (e.key.toLowerCase() === "c" && !e.metaKey && !e.ctrlKey && !e.altKey && !isTyping()) location.href = "/checkout";
});

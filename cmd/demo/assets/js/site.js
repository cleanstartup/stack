(() => {
  const headline = document.getElementById("headline");
  if (!headline) return;

  headline.addEventListener("click", () => {
    headline.textContent = "hello from way2go (clicked)";
  });

  console.log("way2go demo JS loaded");
})();

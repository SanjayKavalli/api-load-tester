"use strict";
(() => {
  const refreshRunStatus = () => {
    if (typeof poll === "function") void poll();
  };
  window.addEventListener("focus", refreshRunStatus);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") refreshRunStatus();
  });
  fetch("/api/ui")
    .then((response) => {
      if (!response.ok) throw new Error("Interface settings unavailable");
      return response.json();
    })
    .then((options) => {
      if (!options.classic || !options.balanced) return;
      const navigation = document.createElement("nav");
      navigation.className = "layout-switch";
      navigation.setAttribute("aria-label", "Workspace interface");
      const balanced = document.body.classList.contains("balanced-page");
      for (const [layout, label] of [
        ["classic", "Classic"],
        ["balanced", "Balanced"],
      ]) {
        const current = (layout === "balanced") === balanced;
        const element = document.createElement(current ? "span" : "a");
        element.textContent = label;
        if (current) element.setAttribute("aria-current", "page");
        else {
          element.href = `/${layout}`;
          element.target = "_blank";
          element.rel = "noopener";
          element.title = `Open ${label} in a separate tab; your current setup stays here`;
        }
        navigation.append(element);
      }
      const brandLockup = document.querySelector("header .brand-lockup");
      (brandLockup ?? document.querySelector("header .brand"))?.after(
        navigation,
      );
    })
    .catch(() => {});
})();

"use strict";
(() => {
  if (!document.body.classList.contains("balanced-page")) return;
  document.title = "Load Lab | Balanced Workspace";
  const workspace = document.querySelector(".workspace");
  const sections = [...document.querySelector(".configuration").children];
  const [traffic, request, rules] = sections;
  const tabs = document.querySelector("#scenario-tabs");
  const preview = document.querySelector(".preview-panel");
  const results = document.querySelector("#results");
  const storageKey = "load-lab-balanced-layout-v1";
  const defaults = { api: 220, settings: 320, console: 310, inspector: 50 };
  let dimensions = { ...defaults };
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || "null");
    for (const key of Object.keys(defaults))
      if (Number.isFinite(saved?.[key])) dimensions[key] = saved[key];
  } catch {}
  function element(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }
  function persist() {
    try {
      localStorage.setItem(storageKey, JSON.stringify(dimensions));
    } catch {}
  }
  function clamp(value, min, max) {
    return Math.max(min, Math.min(value, Math.max(min, max)));
  }
  function limits(key) {
    const width = workspace.clientWidth;
    if (key === "api")
      return [
        140,
        Math.min(
          380,
          width - (innerWidth > 1050 ? dimensions.settings + 380 : 360),
        ),
      ];
    if (key === "settings")
      return [250, Math.min(460, width - dimensions.api - 380)];
    if (key === "console") return [180, Math.max(500, innerHeight * 0.8)];
    return [25, 75];
  }
  function applyDimensions() {
    dimensions.api = clamp(dimensions.api, ...limits("api"));
    dimensions.settings = clamp(dimensions.settings, ...limits("settings"));
    dimensions.console = clamp(dimensions.console, ...limits("console"));
    dimensions.inspector = clamp(dimensions.inspector, 25, 75);
    workspace.style.setProperty("--api-width", `${dimensions.api}px`);
    workspace.style.setProperty("--settings-width", `${dimensions.settings}px`);
    workspace.style.setProperty("--console-height", `${dimensions.console}px`);
    const inspector = document.querySelector("#request-inspector");
    inspector.style.setProperty(
      "--inspector-first",
      `${dimensions.inspector}fr`,
    );
    inspector.style.setProperty(
      "--inspector-second",
      `${100 - dimensions.inspector}fr`,
    );
    document.querySelectorAll("[data-resize]").forEach((handle) => {
      const key = handle.dataset.resize,
        [minimum, maximum] = limits(key);
      handle.setAttribute("aria-valuenow", String(Math.round(dimensions[key])));
      handle.setAttribute(
        "aria-valuetext",
        `${Math.round(dimensions[key])}${key === "inspector" ? "% request width" : " pixels"}`,
      );
      handle.setAttribute("aria-valuemin", String(minimum));
      handle.setAttribute("aria-valuemax", String(Math.round(maximum)));
    });
  }
  function separator(key, label, orientation) {
    const handle = element("div", `balanced-splitter balanced-splitter-${key}`);
    handle.dataset.resize = key;
    handle.tabIndex = 0;
    handle.setAttribute("role", "separator");
    handle.setAttribute("aria-label", label);
    handle.setAttribute("aria-orientation", orientation);
    handle.title = `${label}. Drag or use arrow keys; double-click to reset.`;
    handle.addEventListener("pointerdown", (event) => {
      if (event.button !== 0) return;
      event.preventDefault();
      handle.setPointerCapture(event.pointerId);
      const startX = event.clientX,
        startY = event.clientY,
        start = dimensions[key];
      document.body.classList.add("balanced-resizing");
      const move = (moveEvent) => {
        const delta =
          key === "console"
            ? startY - moveEvent.clientY
            : key === "settings"
              ? startX - moveEvent.clientX
              : key === "inspector"
                ? ((moveEvent.clientX - startX) /
                    Math.max(
                      1,
                      document.querySelector("#request-inspector").clientWidth,
                    )) *
                  100
                : moveEvent.clientX - startX;
        dimensions[key] = clamp(start + delta, ...limits(key));
        applyDimensions();
      };
      const finish = () => {
        handle.removeEventListener("pointermove", move);
        handle.removeEventListener("pointerup", finish);
        handle.removeEventListener("pointercancel", finish);
        document.body.classList.remove("balanced-resizing");
        persist();
      };
      handle.addEventListener("pointermove", move);
      handle.addEventListener("pointerup", finish);
      handle.addEventListener("pointercancel", finish);
    });
    handle.addEventListener("keydown", (event) => {
      let step = event.shiftKey ? 30 : 10;
      if (key === "inspector") step = event.shiftKey ? 10 : 2;
      let direction = 0;
      if (key === "console")
        direction =
          event.key === "ArrowUp" ? 1 : event.key === "ArrowDown" ? -1 : 0;
      else
        direction =
          event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
      if (key === "settings") direction = -direction;
      if (event.key === "Home") dimensions[key] = limits(key)[0];
      else if (event.key === "End") dimensions[key] = limits(key)[1];
      else if (direction) dimensions[key] += direction * step;
      else return;
      event.preventDefault();
      applyDimensions();
      persist();
    });
    handle.addEventListener("dblclick", () => {
      dimensions[key] = defaults[key];
      applyDimensions();
      persist();
    });
    return handle;
  }
  const heading = element("div", "balanced-workspace-heading");
  const headingText = element("div");
  headingText.append(
    element("h1", "", "Balanced workspace"),
    element(
      "span",
      "balanced-subtitle",
      "API requests / traffic / run console",
    ),
  );
  const reset = element("button", "icon-button");
  reset.type = "button";
  reset.title = "Reset pane sizes";
  reset.setAttribute("aria-label", "Reset pane sizes");
  reset.innerHTML = icon("refresh");
  reset.onclick = () => {
    dimensions = { ...defaults };
    applyDimensions();
    persist();
  };
  heading.append(headingText, reset);
  workspace.before(heading);
  document.querySelector("header .brand-tag").textContent = "BALANCED";

  const sidebar = element("aside", "balanced-api-pane");
  const sidebarHeader = element("div", "balanced-api-heading");
  const title = request.querySelector(".section-title h2");
  title.querySelector("span")?.remove();
  [...title.childNodes]
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .forEach((node) => {
      node.textContent = "APIs";
    });
  const add = document.querySelector("#add-scenario");
  add.innerHTML = icon("plus");
  add.title = "Add API";
  add.setAttribute("aria-label", "Add API");
  sidebarHeader.append(title, add);
  const search = element("input", "balanced-api-search");
  search.type = "search";
  search.placeholder = "Filter APIs";
  search.setAttribute("aria-label", "Filter API list");
  search.title =
    "Filters this list only. All configured APIs remain included in the test.";
  const count = element("span", "balanced-api-count");
  count.setAttribute("aria-live", "polite");
  const empty = element("div", "balanced-api-empty", "No matching APIs.");
  empty.hidden = true;
  sidebar.append(sidebarHeader, search, count, tabs, empty);
  workspace.prepend(sidebar);
  tabs.setAttribute("aria-orientation", "vertical");
  function decorateAPIs() {
    const query = search.value.toLowerCase();
    let visible = 0;
    tabs.querySelectorAll("button[data-index]").forEach((button) => {
      const index = Number(button.dataset.index),
        scenario = scenarios[index];
      if (!scenario) return;
      const current = index === active;
      const name = current
        ? document.querySelector("#scenario-name")?.value || scenario.name
        : scenario.name;
      const method = current
        ? document.querySelector("#method")?.value || scenario.method
        : scenario.method;
      const url = current
        ? document.querySelector("#url")?.value || scenario.url
        : scenario.url;
      const weight = current
        ? document.querySelector("#scenario-weight")?.value || scenario.weight
        : scenario.weight;
      let path = url || "No target URL";
      try {
        const parsed = new URL(url);
        path = parsed.pathname + parsed.search;
      } catch {}
      const methodLabel = element(
        "span",
        `balanced-method method-${method.toLowerCase()}`,
        method,
      );
      const nameLabel = element(
        "span",
        "balanced-api-name",
        name || "Unnamed API",
      );
      const pathLabel = element("small", "balanced-api-path", path);
      const allocation = element(
        "span",
        "balanced-api-share",
        allAPIsTogether() ? "" : `${weight}%`,
      );
      button.replaceChildren(methodLabel, nameLabel, allocation, pathLabel);
      button.classList.add("balanced-api-row");
      button.tabIndex = current ? 0 : -1;
      button.title = `${method} ${url || "No target URL"}`;
      button.hidden = !`${name} ${method} ${url}`.toLowerCase().includes(query);
      if (!button.hidden) visible++;
    });
    const visibleButtons = [...tabs.querySelectorAll("button:not([hidden])")];
    if (
      !visibleButtons.some((button) => button.tabIndex === 0) &&
      visibleButtons.length
    )
      visibleButtons[0].tabIndex = 0;
    count.textContent = query
      ? `${visible} of ${scenarios.length} APIs`
      : `${scenarios.length} API${scenarios.length === 1 ? "" : "s"}`;
    empty.hidden = visible !== 0;
  }
  search.oninput = decorateAPIs;
  search.onkeydown = (event) => {
    if (event.key !== "ArrowDown" && event.key !== "Enter") return;
    const first = tabs.querySelector("button:not([hidden])");
    if (!first) return;
    event.preventDefault();
    if (event.key === "Enter") first.click();
    else first.focus();
  };
  tabs.addEventListener("keydown", (event) => {
    const buttons = [...tabs.querySelectorAll("button:not([hidden])")];
    const current = buttons.indexOf(document.activeElement);
    let next = current;
    if (event.key === "ArrowDown") next = (current + 1) % buttons.length;
    else if (event.key === "ArrowUp")
      next = (current - 1 + buttons.length) % buttons.length;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = buttons.length - 1;
    else return;
    if (!buttons.length) return;
    event.preventDefault();
    const index = buttons[next].dataset.index;
    buttons[next].click();
    queueMicrotask(() =>
      tabs.querySelector(`[data-index="${index}"]`)?.focus(),
    );
  });
  new MutationObserver(decorateAPIs).observe(tabs, { childList: true });
  new MutationObserver(decorateAPIs).observe(
    document.querySelector("#scenario-editor"),
    { childList: true },
  );
  const updateAPIMetadata = (event) => {
    if (
      ["scenario-name", "method", "url", "scenario-weight"].includes(
        event.target.id,
      )
    )
      decorateAPIs();
  };
  document
    .querySelector("#scenario-editor")
    .addEventListener("input", updateAPIMetadata);
  document
    .querySelector("#scenario-editor")
    .addEventListener("change", updateAPIMetadata);

  const editorTabs = element("div", "balanced-editor-tabs");
  editorTabs.setAttribute("role", "tablist");
  editorTabs.setAttribute("aria-label", "Request workspace views");
  const panels = { request, rules, preview };
  function selectPanel(name) {
    for (const [key, panel] of Object.entries(panels)) {
      panel.classList.toggle("balanced-panel-active", key === name);
      panel.setAttribute("role", "tabpanel");
      panel.id = `balanced-panel-${key}`;
      panel.setAttribute("aria-labelledby", `balanced-tab-${key}`);
    }
    editorTabs.querySelectorAll("button").forEach((button) => {
      const selected = button.dataset.panel === name;
      button.setAttribute("aria-selected", String(selected));
      button.tabIndex = selected ? 0 : -1;
    });
  }
  for (const [name, label] of [
    ["request", "Request"],
    ["rules", "Field rules"],
    ["preview", "Payload preview"],
  ]) {
    const button = element("button", "", label);
    button.type = "button";
    button.dataset.panel = name;
    button.id = `balanced-tab-${name}`;
    button.setAttribute("role", "tab");
    button.setAttribute("aria-controls", `balanced-panel-${name}`);
    button.onclick = () => selectPanel(name);
    editorTabs.append(button);
  }
  editorTabs.addEventListener("keydown", (event) => {
    const buttons = [...editorTabs.children];
    let index = buttons.indexOf(document.activeElement);
    if (event.key === "ArrowRight") index = (index + 1) % buttons.length;
    else if (event.key === "ArrowLeft")
      index = (index - 1 + buttons.length) % buttons.length;
    else if (event.key === "Home") index = 0;
    else if (event.key === "End") index = buttons.length - 1;
    else return;
    event.preventDefault();
    buttons[index].click();
    buttons[index].focus();
  });
  workspace.append(editorTabs);
  selectPanel("request");

  const pickerLabel = element("label", "balanced-mode-label", "Traffic mode");
  const picker = element("select");
  picker.id = "balanced-profile";
  picker.setAttribute("aria-label", "Traffic mode");
  for (const [value, label] of [
    ["normal", "Normal / steady traffic"],
    ["burst", "Burst / simultaneous users"],
    ["multi_api_burst", "Multi-API Burst / all APIs together"],
    ["weighted", "Weighted / custom percentages"],
    ["coordinated", "Coordinated / shared records"],
  ]) {
    const option = element("option", "", label);
    option.value = value;
    picker.append(option);
  }
  pickerLabel.append(picker);
  traffic.querySelector(".profiles").before(pickerLabel);
  picker.onchange = () => {
    document.querySelector(`[data-profile="${picker.value}"]`).click();
  };
  const synchronizeMode = () => {
    picker.value = profile;
    decorateAPIs();
  };
  new MutationObserver(synchronizeMode).observe(
    traffic.querySelector(".profiles"),
    { attributes: true, subtree: true, attributeFilter: ["class"] },
  );

  const consoleHeading = results.querySelector("h2");
  [...consoleHeading.childNodes]
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .forEach((node) => (node.textContent = "Run console"));
  workspace.append(results);
  workspace.append(
    separator("api", "Resize API sidebar", "vertical"),
    separator("settings", "Resize run settings", "vertical"),
    separator("console", "Resize run console", "horizontal"),
  );
  const inspector = document.querySelector("#request-inspector");
  inspector.insertBefore(
    separator("inspector", "Resize request and response panes", "vertical"),
    inspector.children[1],
  );
  const panelObserver = new MutationObserver(() => {
    if (!inspector.hidden) applyDimensions();
  });
  panelObserver.observe(inspector, {
    attributes: true,
    attributeFilter: ["hidden"],
  });
  window.addEventListener("resize", applyDimensions);
  decorateAPIs();
  synchronizeMode();
  applyDimensions();
})();

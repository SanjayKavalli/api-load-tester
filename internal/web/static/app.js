"use strict";
const $ = (selector) => document.querySelector(selector);
const pretty = (value) => JSON.stringify(value, null, 2);
const escapeHTML = (value) =>
  String(value).replace(
    /[&<>"']/g,
    (char) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        char
      ],
  );
const icon = (name) => `<svg><use href="/icons.svg#${name}"></use></svg>`;
let profile = "normal",
  active = 0,
  sharedData = null,
  previewBodies = null,
  lastReport = null,
  polling = false;
let noticeTimer;
let runBusy = false;
let pollTimer;
let pollGeneration = 0;
let previewRevision = 0;
const helpTopics = {
  traffic: [
    "Traffic profile",
    "Choose steady traffic, a burst, a custom percentage split, or all APIs together.",
    "traffic-profiles",
  ],
  duration: [
    "Duration",
    "How long to send new requests, for example 30s or 2m.",
    "traffic-settings",
  ],
  rate: [
    "Bursts / second",
    "How many groups of users start each second.",
    "traffic-settings",
  ],
  burst: [
    "Users / burst",
    "How many virtual users start together in each group.",
    "traffic-settings",
  ],
  concurrency: [
    "Max in flight",
    "The maximum requests running at once; extra arrivals are dropped.",
    "traffic-settings",
  ],
  timeout: [
    "Request timeout",
    "How long each request can wait for a response, for example 10s.",
    "traffic-settings",
  ],
  sharedMode: [
    "Shared record mode",
    "Use one fixed record, move through records, or pick randomly for coordinated users.",
    "datasets",
  ],
  scenarios: [
    "API scenarios",
    "Add an API for each request you want to include in the test.",
    "api-scenarios",
  ],
  name: [
    "Scenario name",
    "A unique name that identifies this API in the results.",
    "api-scenarios",
  ],
  percentage: [
    "Percentage (%)",
    "This API's share of traffic; all API percentages must add up to 100%.",
    "percentages",
  ],
  endpoint: [
    "API endpoint",
    "Choose the HTTP method and the full URL of your approved test API.",
    "api-scenarios",
  ],
  payload: [
    "Request body",
    "Upload a JSON object or edit the data sent to this API.",
    "payloads",
  ],
  dataset: [
    "Scenario dataset",
    "Upload a JSON array of records to reuse existing test data.",
    "datasets",
  ],
  overlay: [
    "Overlay dataset values",
    "Replace matching payload keys with the selected dataset record before applying rules.",
    "datasets",
  ],
  recordMode: [
    "Record selection",
    "Cycle through dataset records in order, or select a random record.",
    "datasets",
  ],
  rules: [
    "Field rules",
    "Keep values, generate them, or calculate them; rules run from top to bottom.",
    "field-rules",
  ],
  source: [
    "Value source",
    "Choose Keep value, Regex, Expression, or File (base64) for a field.",
    "field-rules",
  ],
  pattern: [
    "Pattern / expression",
    "Enter a regex pattern or a calculation using other payload fields.",
    "rule-examples",
  ],
  headers: [
    "Headers",
    "Add request headers as JSON, such as Content-Type or an API key.",
    "headers",
  ],
  token: [
    "Bearer token",
    "Enter the test token required by this API; treat it as a secret.",
    "headers",
  ],
  statuses: [
    "Expected status codes",
    "List successful HTTP response codes separated by commas, such as 200, 201.",
    "headers",
  ],
  preview: [
    "Payload preview",
    "Generate a sample payload without sending a request to the target API.",
    "preview",
  ],
  running: [
    "Run controls",
    "Confirm authorization and start; Stop ends new scheduling while active requests finish.",
    "running",
  ],
  results: [
    "Run results",
    "Review completed requests, failures, dropped arrivals, and response times.",
    "results",
  ],
  setups: [
    "Saved setups",
    "Import a previously exported Load Lab setup to restore APIs, data, and rules.",
    "saved-setups",
  ],
  burstSchedule: [
    "Burst schedule",
    "Send all APIs once, or repeat complete synchronized groups at the chosen interval.",
    "multi-api-burst",
  ],
  burstInterval: [
    "Repeat interval",
    "Seconds between burst releases; groups are skipped if previous requests still fill the available slots.",
    "multi-api-burst",
  ],
  dataStrategy: [
    "Data strategy",
    "Fresh mode protects marked Unique fields; reuse mode keeps their existing payload or dataset values.",
    "repeated-runs",
  ],
  unique: [
    "Unique fields",
    "Marked fields are reserved across runs on this machine; names can repeat unless you mark them Unique.",
    "repeated-runs",
  ],
  captureBodies: [
    "Record sent request bodies",
    "Keep exact sent JSON beside each response; reports may contain sensitive data.",
    "request-bodies",
  ],
  samples: [
    "Samples",
    "Generate several example bodies without sending requests or consuming persistent Unique values.",
    "request-bodies",
  ],
};
const helpPanel = document.createElement("div");
helpPanel.id = "help-popover";
helpPanel.className = "help-popover";
helpPanel.setAttribute("popover", "manual");
helpPanel.setAttribute("role", "dialog");
helpPanel.setAttribute("aria-labelledby", "help-title");
helpPanel.innerHTML =
  '<div class="help-heading"><strong id="help-title"></strong><button class="help-close" type="button" aria-label="Close help">x</button></div><p id="help-description"></p><a id="help-manual-link" href="/user-manual.html" target="_blank" rel="noopener">Read more</a>';
document.body.append(helpPanel);
let helpAnchor = null;
let helpCloseTimer;
function closeHelp() {
  clearTimeout(helpCloseTimer);
  helpPanel.hidePopover();
  helpAnchor?.setAttribute("aria-expanded", "false");
  helpAnchor = null;
}
function positionHelp() {
  if (!helpAnchor || !helpAnchor.isConnected) {
    closeHelp();
    return;
  }
  const anchor = helpAnchor.getBoundingClientRect();
  const panel = helpPanel.getBoundingClientRect();
  const left = Math.max(
    12,
    Math.min(anchor.left, innerWidth - panel.width - 12),
  );
  const below = anchor.bottom + 8;
  const top =
    below + panel.height <= innerHeight - 12
      ? below
      : Math.max(12, anchor.top - panel.height - 8);
  helpPanel.style.left = `${left}px`;
  helpPanel.style.top = `${top}px`;
}
function openHelp(button) {
  clearTimeout(helpCloseTimer);
  const [title, description, section] = helpTopics[button.dataset.help];
  helpAnchor?.setAttribute("aria-expanded", "false");
  helpAnchor = button;
  $("#help-title").textContent = title;
  $("#help-description").textContent = description;
  $("#help-manual-link").href = `/user-manual.html#${section}`;
  button.setAttribute("aria-expanded", "true");
  helpPanel.showPopover();
  positionHelp();
}
function installHelp() {
  const targets = [
    [".configuration .band:nth-child(1) h2", "traffic"],
    ["#duration", "duration"],
    ["#rate", "rate"],
    ["#burst", "burst"],
    ["#concurrency", "concurrency"],
    ["#timeout", "timeout"],
    ["#shared-mode", "sharedMode"],
    ["#shared-file", "dataset"],
    [".configuration .band:nth-child(2) h2", "scenarios"],
    ["#scenario-name", "name"],
    ["#scenario-weight", "percentage"],
    [".request-row", "endpoint"],
    ["label[for='body-editor']", "payload"],
    ["#dataset-file", "dataset"],
    ["#overlay", "overlay"],
    ["#dataset-mode", "recordMode"],
    ["label[for='headers-editor']", "headers"],
    ["#bearer-token", "token"],
    ["#statuses", "statuses"],
    [".configuration .band:nth-child(3) h2", "rules"],
    [".rule-table th:nth-child(2)", "source"],
    [".rule-table th:nth-child(3)", "pattern"],
    [".preview-panel h2", "preview"],
    [".run-panel h2", "running"],
    ["#results h2", "results"],
    [".import-config", "setups"],
    ["#burst-mode", "burstSchedule"],
    ["#burst-interval", "burstInterval"],
    ["#data-mode", "dataStrategy"],
    ["#record-bodies", "captureBodies"],
    ["#sample-count", "samples"],
    ["#field-count", "unique"],
  ];
  for (const [selector, topic] of targets) {
    const element = $(selector);
    if (!element) continue;
    let target = element.matches("input, select")
      ? element.closest("label")
      : element;
    if (!target || target.querySelector(`.help-button[data-help='${topic}']`))
      continue;
    if (target.tagName === "LABEL") {
      const control = element.matches("input, select")
        ? element
        : target.querySelector("input, select, textarea");
      if (control?.id) target.htmlFor = control.id;
      const text = [...target.childNodes].find(
        (node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim(),
      );
      if (text) {
        const caption = document.createElement("span");
        caption.className = "label-heading";
        caption.textContent = text.textContent.trim();
        target.replaceChild(caption, text);
        target = caption;
      }
      if (
        element.matches("input, select") &&
        !element.hasAttribute("aria-label")
      )
        element.setAttribute("aria-label", helpTopics[topic][0]);
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "help-button";
    button.dataset.help = topic;
    button.setAttribute("aria-label", `Help: ${helpTopics[topic][0]}`);
    button.setAttribute("aria-expanded", "false");
    button.setAttribute("aria-controls", "help-popover");
    button.setAttribute("aria-haspopup", "dialog");
    button.textContent = "?";
    target.append(button);
  }
}
document.addEventListener("pointerover", (event) => {
  const button = event.target.closest(".help-button");
  if (button && event.pointerType !== "touch") openHelp(button);
});
document.addEventListener("pointerout", (event) => {
  if (
    event.target.closest(".help-button") ||
    helpPanel.contains(event.target)
  ) {
    clearTimeout(helpCloseTimer);
    helpCloseTimer = setTimeout(() => {
      if (
        !helpPanel.contains(document.activeElement) &&
        document.activeElement !== helpAnchor
      )
        closeHelp();
    }, 250);
  }
});
helpPanel.addEventListener("pointerover", () => clearTimeout(helpCloseTimer));
document.addEventListener("focusin", (event) => {
  const button = event.target.closest(".help-button");
  if (button) openHelp(button);
  else if (!helpPanel.contains(event.target)) closeHelp();
});
document.addEventListener("click", (event) => {
  const button = event.target.closest(".help-button");
  if (button) {
    event.preventDefault();
    openHelp(button);
    if (event.detail === 0) $("#help-manual-link").focus();
  } else if (event.target.closest(".help-close")) {
    const anchor = helpAnchor;
    closeHelp();
    anchor?.focus();
    closeHelp();
  } else if (!helpPanel.contains(event.target)) closeHelp();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && helpAnchor) {
    const anchor = helpAnchor;
    closeHelp();
    anchor.focus();
    closeHelp();
    event.preventDefault();
  }
});
window.addEventListener("resize", () => {
  if (helpAnchor) positionHelp();
});
window.addEventListener(
  "scroll",
  () => {
    if (helpAnchor) positionHelp();
  },
  true,
);
function newScenario(name = "Request", weight = 100) {
  return {
    name,
    weight,
    method: "POST",
    url: "",
    body: {
      first_name: "",
      last_name: "",
      patient_id: "",
      gender: "female",
      dob: "1995-04-15",
    },
    rules: [
      { path: ["first_name"], regex: "[A-Z][a-z]{5,9}" },
      { path: ["last_name"], regex: "[A-Z][a-z]{5,9}" },
      { path: ["patient_id"], regex: "PT-[0-9]{8}", unique: true },
    ],
    headers: { "Content-Type": "application/json" },
    statuses: [200, 201],
    dataset: null,
    datasetMode: "cycle",
    overlay: true,
  };
}
let scenarios = [newScenario()];
function notify(message, error = false) {
  clearTimeout(noticeTimer);
  $("#notice").textContent = message;
  $("#notice").className = error ? "error" : "";
  $("#notice").hidden = false;
  noticeTimer = setTimeout(
    () => {
      $("#notice").hidden = true;
    },
    error ? 10000 : 4000,
  );
}
function guard(action) {
  return async (event) => {
    try {
      await action(event);
    } catch (error) {
      notify(error.message, true);
    }
  };
}
function parseObject(text, label) {
  const value = LoadLabUI.parseJSON(text);
  if (!value || Array.isArray(value) || typeof value !== "object")
    throw new Error(`${label} must be a JSON object.`);
  return value;
}
function parseDataset(value) {
  if (
    !Array.isArray(value) ||
    !value.length ||
    value.some(
      (record) =>
        !record || Array.isArray(record) || typeof record !== "object",
    )
  )
    throw new Error("Dataset must be a non-empty JSON array of objects.");
  return value;
}
async function readFile(file, limit = 6 * 1024 * 1024) {
  if (!file) return null;
  if (file.size > limit)
    throw new Error(`File exceeds ${limit / 1024 / 1024} MiB.`);
  return LoadLabUI.parseJSON(await file.text());
}
function flatten(value, path = [], output = []) {
  if (value && typeof value === "object")
    Object.entries(value).forEach(([key, child]) =>
      flatten(child, [...path, key], output),
    );
  else output.push({ path, value });
  return output;
}
function pathKey(path) {
  return JSON.stringify(path);
}
function invalidatePreview() {
  previewRevision++;
  previewBodies = null;
  $("#preview-state").textContent = "Preview out of date";
  $("#download-samples").disabled = true;
}
function capture(ignoreBody = false) {
  const scenario = scenarios[active];
  const body = ignoreBody
    ? scenario.body
    : parseObject($("#body-editor").value, "Payload");
  const headers = parseObject($("#headers-editor").value, "Headers");
  if (Object.values(headers).some((value) => typeof value !== "string"))
    throw new Error("Header values must be strings.");
  scenario.name = $("#scenario-name").value.trim();
  scenario.weight = Number($("#scenario-weight").value);
  scenario.method = $("#method").value;
  scenario.url = $("#url").value.trim();
  scenario.body = body;
  scenario.headers = headers;
  if (
    Object.values(scenario.headers).some((value) => typeof value !== "string")
  )
    throw new Error("Header values must be strings.");
  scenario.statuses = $("#statuses")
    .value.split(",")
    .filter((value) => value.trim())
    .map((value) => Number(value.trim()));
  if (
    scenario.statuses.some(
      (value) => !Number.isInteger(value) || value < 100 || value > 599,
    )
  )
    throw new Error(
      "Expected statuses must be HTTP codes separated by commas.",
    );
  scenario.overlay = $("#overlay").checked;
  scenario.datasetMode = $("#dataset-mode").value;
}
function allAPIsTogether() {
  return profile === "coordinated" || profile === "multi_api_burst";
}
function allocationError() {
  for (const [selector, minimum, maximum, name] of [
    ["#rate", 1, 10000, "Bursts per second"],
    ["#burst", 1, 100000, "Users per burst"],
    [
      "#concurrency",
      1,
      profile === "multi_api_burst" ? 10000 : 1000,
      "Max in flight",
    ],
  ]) {
    const input = $(selector),
      value = Number(input.value);
    const invalid =
      !Number.isInteger(value) || value < minimum || value > maximum;
    input.setAttribute("aria-invalid", String(invalid));
    if (invalid)
      return `${name} must be a whole number between ${minimum} and ${maximum}.`;
  }
  for (const [selector, maximum, name] of [
    ["#duration", 3600, "Duration"],
    ["#timeout", 60, "Request timeout"],
  ]) {
    try {
      const seconds = LoadLabUI.durationSeconds($(selector).value);
      if (seconds <= 0 || seconds > maximum)
        return `${name} must be positive and at most ${maximum} seconds.`;
    } catch (error) {
      return error.message;
    }
  }
  if (profile === "multi_api_burst") {
    const needed = Number($("#burst").value) * scenarios.length;
    if (needed > 10000)
      return "Multi-API Burst supports at most 10,000 requests per burst.";
    if (Number($("#concurrency").value) < needed)
      return `Max in flight must be at least ${needed} for the complete burst.`;
    if (
      $("#burst-mode").value === "repeat" &&
      !(Number($("#burst-interval").value) >= 0.001)
    )
      return "Repeat interval must be at least 0.001 seconds.";
    return "";
  }
  if (allAPIsTogether()) return "";
  if (
    scenarios.some(
      (scenario) =>
        !Number.isInteger(scenario.weight) ||
        scenario.weight < 1 ||
        scenario.weight > 100,
    )
  ) {
    return "Each API percentage must be a whole number between 1 and 100.";
  }
  const total = scenarios.reduce((sum, scenario) => sum + scenario.weight, 0);
  return total === 100
    ? ""
    : `API percentages must total 100% (currently ${total}%).`;
}
function updateTraffic() {
  const factor = allAPIsTogether() ? scenarios.length : 1;
  const rate = Number($("#rate").value) * Number($("#burst").value) * factor;
  $("#traffic-total").textContent =
    profile === "multi_api_burst"
      ? `${(Number($("#burst").value) * factor).toLocaleString()} requests / burst${$("#burst-mode").value === "repeat" ? ` / every ${$("#burst-interval").value}s` : " / once"}`
      : `${rate.toLocaleString()} planned requests / second`;
  const total = scenarios.reduce((sum, scenario) => sum + scenario.weight, 0);
  const error = allocationError();
  $("#weight-total").textContent =
    error ||
    (allAPIsTogether()
      ? error
      : `${total}% / 100%${total < 100 ? ` / ${100 - total}% remaining` : total > 100 ? ` / ${total - 100}% over` : ""}`);
  $("#weight-total").classList.toggle("allocation-error", Boolean(error));
  $("#weight-total").title = error;
  $("#scenario-weight")?.setAttribute("aria-invalid", String(Boolean(error)));
  $("#start").disabled = runBusy || Boolean(error);
  $("#start").title = error || "Start test";
  $("#shared-upload").hidden = !allAPIsTogether();
  $("#shared-mode").disabled = !allAPIsTogether();
  $("#clear-shared").hidden = !sharedData;
  const multi = profile === "multi_api_burst";
  $("#concurrency").max = multi ? "10000" : "1000";
  $("#rate").closest("label").hidden = multi;
  $("#duration").closest("label").hidden =
    multi && $("#burst-mode").value === "once";
  $("#burst-mode-field").hidden = !multi;
  $("#burst-interval-field").hidden =
    !multi || $("#burst-mode").value !== "repeat";
  $("#match-burst").hidden = !multi;
}
function renderTabs() {
  $("#scenario-tabs").innerHTML = scenarios
    .map(
      (scenario, index) =>
        `<button role="tab" type="button" aria-selected="${index === active}" class="${index === active ? "active" : ""}" data-index="${index}">${escapeHTML(scenario.name || "Unnamed")}${!allAPIsTogether() ? ` <span> / ${escapeHTML(scenario.weight)}%</span>` : ""}</button>`,
    )
    .join("");
  $("#scenario-tabs")
    .querySelectorAll("button")
    .forEach(
      (button) =>
        (button.onclick = guard(() => {
          capture();
          active = Number(button.dataset.index);
          render();
        })),
    );
}
function render() {
  closeHelp();
  const scenario = scenarios[active];
  renderTabs();
  $("#scenario-editor").innerHTML = `
    <div class="scenario-top"><label class="field">Scenario name<input id="scenario-name" value="${escapeHTML(scenario.name)}" maxlength="80"></label><label class="field weight" ${allAPIsTogether() ? "hidden" : ""}>Percentage (%)<input id="scenario-weight" type="number" min="1" max="100" step="1" value="${escapeHTML(scenario.weight)}"></label><button id="remove-scenario" class="icon-button" type="button" title="Remove scenario" aria-label="Remove scenario" ${scenarios.length === 1 ? "disabled" : ""}>${icon("trash")}</button></div>
    <div class="request-row"><select id="method" aria-label="HTTP method">${["POST", "PUT", "PATCH", "GET", "DELETE", "HEAD", "OPTIONS"].map((method) => `<option ${method === scenario.method ? "selected" : ""}>${method}</option>`).join("")}</select><input id="url" type="url" aria-label="Target URL" placeholder="https://staging.example.com/api/patients" value="${escapeHTML(scenario.url)}"></div>
    <button class="upload-zone" id="choose-payload" type="button">${icon("upload")}<span>${escapeHTML(scenario.filename || "Upload request payload")}<small>JSON request body</small></span></button><input id="payload-file" type="file" accept=".json,application/json" aria-label="Request payload file" hidden>
    <label class="field" for="body-editor">Request body</label><textarea id="body-editor" class="payload-editor" spellcheck="false" aria-label="Request body">${escapeHTML(pretty(scenario.body))}</textarea>
    <div class="details-row"><label class="check"><input id="overlay" type="checkbox" ${scenario.overlay ? "checked" : ""}>Overlay dataset values</label><span class="muted" id="dataset-count">${scenario.dataset ? `${scenario.dataset.length} records` : "No dataset"}</span></div>
    <label class="file-line">Scenario dataset<input id="dataset-file" type="file" accept=".json,application/json"><button id="clear-dataset" type="button" class="text-button" ${scenario.dataset ? "" : "hidden"}>Clear</button></label>
    <div class="input-grid"><label>Record selection<select id="dataset-mode"><option value="cycle" ${scenario.datasetMode === "cycle" ? "selected" : ""}>Cycle records</option><option value="random" ${scenario.datasetMode === "random" ? "selected" : ""}>Random record</option></select></label></div>
    <details><summary>Headers & response settings</summary><label class="field" for="headers-editor">Headers (JSON)</label><textarea id="headers-editor" aria-label="Headers JSON" spellcheck="false">${escapeHTML(pretty(scenario.headers))}</textarea><div class="headers-grid"><label class="field">Bearer token<input id="bearer-token" type="password" autocomplete="off" placeholder="Optional; stored in headers on export"></label><label class="field">Expected status codes<input id="statuses" value="${escapeHTML(scenario.statuses.join(", "))}" placeholder="200, 201"></label></div></details>`;
  $("#preview-name").textContent = scenario.name || "REQUEST BODY";
  displayPreview();
  $("#choose-payload").onclick = () => $("#payload-file").click();
  $("#payload-file").onchange = guard(async (event) => {
    const file = event.target.files[0];
    if (!file) return;
    const value = await readFile(file);
    capture(true);
    scenario.body = parseObject(JSON.stringify(value), "Payload");
    scenario.filename = file.name;
    scenario.rules = [];
    invalidatePreview();
    render();
  });
  $("#body-editor").onchange = guard(() => {
    capture();
    invalidatePreview();
    renderRules();
  });
  $("#dataset-file").onchange = guard(async (event) => {
    capture();
    scenario.dataset = parseDataset(await readFile(event.target.files[0]));
    invalidatePreview();
    render();
  });
  $("#clear-dataset").onclick = guard(() => {
    capture();
    scenario.dataset = null;
    invalidatePreview();
    render();
  });
  $("#remove-scenario").onclick = guard(() => {
    scenarios.splice(active, 1);
    active = Math.max(0, active - 1);
    invalidatePreview();
    render();
  });
  $("#bearer-token").onchange = guard(() => {
    const headers = parseObject($("#headers-editor").value, "Headers");
    Object.keys(headers)
      .filter((key) => key.toLowerCase() === "authorization")
      .forEach((key) => delete headers[key]);
    const token = $("#bearer-token").value.trim();
    if (token)
      headers.Authorization = /^bearer /i.test(token)
        ? token
        : `Bearer ${token}`;
    $("#headers-editor").value = pretty(headers);
    invalidatePreview();
  });
  ["#scenario-name", "#scenario-weight"].forEach(
    (selector) =>
      ($(selector).onchange = guard(() => {
        capture();
        invalidatePreview();
        renderTabs();
        updateTraffic();
      })),
  );
  $("#scenario-weight").oninput = () => {
    scenario.weight = Number($("#scenario-weight").value);
    invalidatePreview();
    renderTabs();
    updateTraffic();
  };
  $("#scenario-editor").addEventListener("input", invalidatePreview);
  $("#dataset-file").disabled = allAPIsTogether();
  $("#dataset-mode").disabled = allAPIsTogether();
  if (allAPIsTogether() && scenario.dataset)
    $("#dataset-count").textContent =
      "Scenario dataset not used in this profile";
  renderRules();
  updateTraffic();
  installHelp();
}
function renderRules() {
  const scenario = scenarios[active];
  const fields = flatten(scenario.body);
  const existing = new Set(fields.map((field) => pathKey(field.path)));
  scenario.rules = scenario.rules.filter((rule) =>
    existing.has(pathKey(rule.path)),
  );
  fields.sort((left, right) => {
    const position = (field) => {
      const index = scenario.rules.findIndex(
        (rule) => pathKey(rule.path) === pathKey(field.path),
      );
      return index < 0 ? 10000 : index;
    };
    return position(left) - position(right);
  });
  $("#field-count").textContent =
    `${fields.length} fields / ${scenario.rules.length} rules`;
  if (!fields.length) {
    $("#field-rules").innerHTML = '<div class="empty">No scalar fields.</div>';
    return;
  }
  $("#field-rules").innerHTML =
    `<table class="rule-table"><thead><tr><th>FIELD</th><th>VALUE SOURCE</th><th>PATTERN / EXPRESSION</th></tr></thead><tbody>${fields
      .map((field, index) => {
        const rule = scenario.rules.find(
          (rule) => pathKey(rule.path) === pathKey(field.path),
        );
        const mode = rule?.regex
          ? "regex"
          : rule?.expression
            ? "expression"
            : "keep";
        const reused = rule?.unique && $("#data-mode").value === "existing";
        return `<tr><td title="${escapeHTML(pathKey(field.path))}">${escapeHTML(field.path.join("."))}<small>${field.value === null ? "null" : typeof field.value}</small>${rule ? `<label class="rule-unique"><input type="checkbox" data-unique="${index}" aria-label="Unique for ${escapeHTML(field.path.join("."))}" ${rule.unique ? "checked" : ""} ${$("#data-mode").value === "existing" ? "disabled" : ""}>${reused ? "Reused" : "Unique"}</label><div class="rule-order"><button data-up="${index}" type="button" title="Move rule earlier" aria-label="Move ${escapeHTML(field.path.join("."))} rule earlier">&#8593;</button><button data-down="${index}" type="button" title="Move rule later" aria-label="Move ${escapeHTML(field.path.join("."))} rule later">&#8595;</button></div>` : ""}</td><td><select class="rule-mode" aria-label="Value source for ${escapeHTML(field.path.join("."))}" data-field="${index}"><option value="keep" ${mode === "keep" ? "selected" : ""}>Keep value</option><option value="regex" ${mode === "regex" ? "selected" : ""}>Regex</option><option value="expression" ${mode === "expression" ? "selected" : ""}>Expression</option>${typeof field.value === "string" ? '<option value="file">File (base64)</option>' : ""}</select></td><td><input class="rule-input" aria-label="Rule for ${escapeHTML(field.path.join("."))}" data-field="${index}" value="${escapeHTML(rule?.regex || rule?.expression || "")}" ${mode === "keep" || reused ? "disabled" : ""} placeholder="${mode === "expression" ? "body.first_name + '-test'" : mode === "regex" ? "[A-Z]{3}[0-9]{6}" : "Original payload value"}"></td></tr>`;
      })
      .join("")}</tbody></table>`;
  installHelp();
  $("#field-rules")
    .querySelectorAll("[data-unique]")
    .forEach(
      (input) =>
        (input.onchange = () => {
          const field = fields[Number(input.dataset.unique)];
          const rule = scenario.rules.find(
            (rule) => pathKey(rule.path) === pathKey(field.path),
          );
          rule.unique = input.checked;
          invalidatePreview();
        }),
    );
  $("#field-rules")
    .querySelectorAll(".rule-mode")
    .forEach(
      (select) =>
        (select.onchange = () => {
          const field = fields[Number(select.dataset.field)];
          scenario.rules = scenario.rules.filter(
            (rule) => pathKey(rule.path) !== pathKey(field.path),
          );
          if (select.value === "file") {
            const cell = select.closest("tr").lastElementChild;
            cell.innerHTML =
              '<input type="file" class="rule-input" aria-label="Base64 attachment">';
            cell.querySelector("input").onchange = guard(async (event) => {
              const file = event.target.files[0];
              if (!file) return;
              if (file.size > 4 * 1024 * 1024)
                throw new Error("Attachment exceeds 4 MB.");
              const data = await new Promise((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = () => resolve(reader.result);
                reader.onerror = () =>
                  reject(new Error("Could not read attachment."));
                reader.readAsDataURL(file);
              });
              const base64 = data.slice(data.indexOf(",") + 1);
              let parent = scenario.body;
              field.path.slice(0, -1).forEach((key) => {
                parent = parent[key];
              });
              parent[field.path.at(-1)] = /\{\{file_base64:[^{}]+\}\}/.test(
                field.value,
              )
                ? field.value.replace(/\{\{file_base64:[^{}]+\}\}/g, base64)
                : base64;
              $("#body-editor").value = pretty(scenario.body);
              invalidatePreview();
              renderRules();
              notify("Attachment embedded.");
            });
            invalidatePreview();
            return;
          }
          if (select.value !== "keep")
            scenario.rules.push({
              path: field.path,
              [select.value]:
                select.value === "regex"
                  ? "[A-Z]{6}"
                  : `body[${JSON.stringify(field.path[0])}]`,
            });
          invalidatePreview();
          renderRules();
        }),
    );
  $("#field-rules")
    .querySelectorAll(".rule-input")
    .forEach(
      (input) =>
        (input.oninput = () => {
          const field = fields[Number(input.dataset.field)];
          const rule = scenario.rules.find(
            (rule) => pathKey(rule.path) === pathKey(field.path),
          );
          if (rule)
            rule["regex" in rule ? "regex" : "expression"] = input.value;
          invalidatePreview();
        }),
    );
  $("#field-rules")
    .querySelectorAll("[data-up],[data-down]")
    .forEach(
      (button) =>
        (button.onclick = () => {
          const field =
            fields[Number(button.dataset.up ?? button.dataset.down)];
          const index = scenario.rules.findIndex(
            (rule) => pathKey(rule.path) === pathKey(field.path),
          );
          const next = index + (button.dataset.up !== undefined ? -1 : 1);
          if (next >= 0 && next < scenario.rules.length)
            [scenario.rules[index], scenario.rules[next]] = [
              scenario.rules[next],
              scenario.rules[index],
            ];
          invalidatePreview();
          renderRules();
        }),
    );
}
function buildSubmission() {
  capture();
  const error = allocationError();
  if (error) throw new Error(error);
  const files = {};
  const names = new Set();
  const config = {
    duration: $("#duration").value,
    rate: Number($("#rate").value),
    burst_size: Number($("#burst").value),
    max_inflight: Number($("#concurrency").value),
    timeout: $("#timeout").value,
    execution_mode: allAPIsTogether() ? profile : "weighted",
    burst_mode: $("#burst-mode").value,
    burst_interval: `${Number($("#burst-interval").value)}s`,
    data_mode: $("#data-mode").value,
    record_request_bodies: $("#record-bodies").checked,
    shared_dataset_mode: $("#shared-mode").value,
    scenarios: [],
  };
  scenarios.forEach((scenario, index) => {
    if (!scenario.name || names.has(scenario.name))
      throw new Error("Each scenario needs a unique name.");
    names.add(scenario.name);
    if (!Number.isInteger(scenario.weight) || scenario.weight < 1)
      throw new Error("Scenario weights must be positive integers.");
    if (!/^https?:\/\//i.test(scenario.url))
      throw new Error(`Enter an HTTP(S) target URL for ${scenario.name}.`);
    const bodyFile = `body-${index + 1}.json`;
    files[bodyFile] = scenario.body;
    const item = {
      name: scenario.name,
      weight: scenario.weight,
      method: scenario.method,
      url: scenario.url,
      body_file: bodyFile,
      headers: scenario.headers,
      overlay_dataset_keys: scenario.overlay,
      expected_statuses: scenario.statuses,
      field_rules: scenario.rules,
    };
    if (scenario.dataset) {
      item.dataset_file = `dataset-${index + 1}.json`;
      item.dataset_mode = scenario.datasetMode;
      files[item.dataset_file] = scenario.dataset;
    }
    config.scenarios.push(item);
  });
  if (sharedData) {
    config.shared_dataset_file = "shared.json";
    files["shared.json"] = sharedData;
  }
  return LoadLabUI.validateSetup({ config, files, profile, version: 1 });
}
async function api(path, submission) {
  const response = await fetch(path, {
    method: submission ? "POST" : "GET",
    headers: submission ? { "Content-Type": "application/json" } : {},
    body: submission ? JSON.stringify(submission) : undefined,
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || "Request failed.");
  return result;
}
function requestSubmission() {
  const { config, files } = buildSubmission();
  return { config, files };
}
function download(value, name, formatted = true) {
  const url = URL.createObjectURL(
    new Blob([formatted ? pretty(value) : JSON.stringify(value)], {
      type: "application/json",
    }),
  );
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function displayPreview() {
  const generated = previewBodies?.[scenarios[active].name];
  const samples = generated
    ? Array.isArray(generated)
      ? generated
      : [generated]
    : [];
  const selected = Math.min(
    Number($("#preview-sample").value) || 0,
    Math.max(0, samples.length - 1),
  );
  $("#preview-sample").innerHTML = Array.from(
    { length: Math.max(1, samples.length) },
    (_, index) => `<option value="${index}">Sample ${index + 1}</option>`,
  ).join("");
  $("#preview-sample").value = selected;
  $("#preview-sample").disabled = !samples.length;
  $("#preview-output").textContent = pretty(
    samples[selected] || scenarios[active].body,
  );
}
$("#preview-sample").onchange = displayPreview;
$("#download-samples").onclick = () => {
  if (previewBodies) download(previewBodies, "generated-request-samples.json");
};
$("#preview").onclick = guard(async () => {
  $("#preview").disabled = true;
  try {
    const revision = previewRevision;
    const generated = await api("/api/preview", {
      ...requestSubmission(),
      sample_count: Number($("#sample-count").value),
    });
    if (revision !== previewRevision) {
      notify("The setup changed. Generate a new preview.");
      return;
    }
    previewBodies = generated;
    displayPreview();
    $("#download-samples").disabled = false;
    $("#preview-state").textContent = "Generated just now";
  } finally {
    $("#preview").disabled = false;
  }
});
$("#copy-preview").onclick = guard(async () => {
  await navigator.clipboard.writeText($("#preview-output").textContent);
  notify("Payload copied.");
});
$("#add-scenario").onclick = guard(() => {
  capture();
  const remaining =
    100 - scenarios.reduce((sum, scenario) => sum + scenario.weight, 0);
  scenarios.push(
    newScenario(`Request ${scenarios.length + 1}`, Math.max(1, remaining)),
  );
  active = scenarios.length - 1;
  invalidatePreview();
  render();
});
document.querySelectorAll("[data-profile]").forEach(
  (button) =>
    (button.onclick = guard(() => {
      capture();
      profile = button.dataset.profile;
      document
        .querySelectorAll("[data-profile]")
        .forEach((item) => item.classList.toggle("selected", item === button));
      if (profile === "normal") $("#burst").value = 1;
      if (
        profile === "burst" ||
        profile === "coordinated" ||
        profile === "multi_api_burst"
      ) {
        $("#burst").value = 100;
        $("#concurrency").value = 100;
        if (profile === "multi_api_burst")
          $("#concurrency").value = Math.min(10000, 100 * scenarios.length);
      }
      if (profile === "weighted") {
        $("#burst").value = 1;
      }
      invalidatePreview();
      render();
    })),
);
[
  "#duration",
  "#rate",
  "#burst",
  "#concurrency",
  "#timeout",
  "#shared-mode",
  "#burst-mode",
  "#burst-interval",
  "#sample-count",
].forEach((selector) =>
  $(selector).addEventListener("input", () => {
    invalidatePreview();
    updateTraffic();
  }),
);
$("#data-mode").onchange = () => {
  invalidatePreview();
  renderRules();
};
$("#match-burst").onclick = () => {
  $("#concurrency").value = Math.min(
    10000,
    Number($("#burst").value) * scenarios.length,
  );
  updateTraffic();
};
$("#shared-file").onchange = guard(async (event) => {
  sharedData = parseDataset(await readFile(event.target.files[0]));
  $("#shared-count").textContent = `${sharedData.length} records`;
  invalidatePreview();
  updateTraffic();
});
$("#clear-shared").onclick = () => {
  sharedData = null;
  $("#shared-file").value = "";
  $("#shared-count").textContent = "";
  invalidatePreview();
  updateTraffic();
};
$("#export").onclick = guard(() => {
  const setup = structuredClone(buildSubmission());
  if (!$("#export-secrets").checked)
    setup.config.scenarios.forEach((scenario) => {
      for (const name of Object.keys(scenario.headers || {}))
        if (/authorization|cookie|token|secret|api[-_]?key/i.test(name))
          delete scenario.headers[name];
    });
  download(setup, "load-lab-setup.json", false);
  notify(
    $("#export-secrets").checked
      ? "Setup exported with credentials. Store securely."
      : "Setup exported without credential headers. Payload data is retained.",
  );
});
$("#import-config").onchange = guard(async (event) => {
  const file = event.target.files[0];
  event.target.value = "";
  if (!file) return;
  const input = LoadLabUI.validateSetup(await readFile(file, 16 * 1024 * 1024));
  await api("/api/validate", { config: input.config, files: input.files });
  if (!input?.config?.scenarios?.length || !input.files)
    throw new Error("Choose an exported Load Lab setup (config + files).");
  const config = input.config;
  const imported = config.scenarios.map((item) => {
    if (!(item.body_file in input.files))
      throw new Error(`Missing payload ${item.body_file}.`);
    return {
      name: item.name,
      weight: item.weight,
      method: item.method || "POST",
      url: item.url,
      body: parseObject(pretty(input.files[item.body_file]), "Payload"),
      rules: item.field_rules || [],
      headers: item.headers || {},
      statuses: item.expected_statuses || [],
      dataset: item.dataset_file
        ? parseDataset(input.files[item.dataset_file])
        : null,
      datasetMode: item.dataset_mode || "cycle",
      overlay: item.overlay_dataset_keys !== false,
      filename: item.body_file,
    };
  });
  sharedData = config.shared_dataset_file
    ? parseDataset(input.files[config.shared_dataset_file])
    : null;
  scenarios = imported;
  active = 0;
  profile =
    input.profile ||
    (["coordinated", "multi_api_burst"].includes(config.execution_mode)
      ? config.execution_mode
      : scenarios.length > 1
        ? "weighted"
        : config.burst_size > 1
          ? "burst"
          : "normal");
  for (const [selector, key, fallback] of [
    ["#duration", "duration", "30s"],
    ["#rate", "rate", 1],
    ["#burst", "burst_size", 1],
    ["#concurrency", "max_inflight", 10],
    ["#timeout", "timeout", "10s"],
    ["#shared-mode", "shared_dataset_mode", "fixed"],
    ["#burst-mode", "burst_mode", "once"],
    ["#data-mode", "data_mode", "fresh"],
  ])
    $(selector).value = config[key] ?? fallback;
  $("#burst-interval").value = parseIntervalSeconds(
    config.burst_interval || "1s",
  );
  $("#record-bodies").checked = Boolean(config.record_request_bodies);
  $("#authorized").checked = false;
  $("#shared-count").textContent = sharedData
    ? `${sharedData.length} records`
    : "";
  document
    .querySelectorAll("[data-profile]")
    .forEach((button) =>
      button.classList.toggle("selected", button.dataset.profile === profile),
    );
  invalidatePreview();
  render();
  notify("Setup imported.");
});
function renderReport(report) {
  if (!report) return;
  lastReport = report;
  $("#download-results").disabled = false;
  document.querySelector('[data-analysis="results"]').disabled = false;
  const summary = report.summary;
  const values = [
    summary.completed,
    summary.failed,
    summary.dropped_arrivals,
    `${(summary.elapsed_ms / 1000).toFixed(2)}s`,
  ];
  document.querySelectorAll("#metrics strong").forEach((item, index) => {
    item.textContent = values[index];
  });
  const responses = report.responses || [];
  const durations = responses
    .map((response) => response.duration_ms)
    .sort((left, right) => left - right);
  const percentile = (value) =>
    durations.length
      ? durations[
          Math.min(
            durations.length - 1,
            Math.ceil(durations.length * value) - 1,
          )
        ].toFixed(2)
      : "0.00";
  $("#result-detail").innerHTML =
    `<div>Run: ${escapeHTML(summary.run_id || "--")} / ${escapeHTML(summary.outcome || "finished")}</div><div>Scheduled: ${summary.scheduled ?? "--"} / Generated: ${summary.generated ?? "--"} / Sent: ${summary.sent ?? summary.completed}</div><div>p95 ${percentile(0.95)} ms &nbsp; / &nbsp; p99 ${percentile(0.99)} ms &nbsp; / &nbsp; Network errors: ${summary.network_errors} &nbsp; / &nbsp; Body errors: ${summary.body_generation_errors}</div><table aria-label="Per-API summary"><thead><tr><th>API</th><th>Completed</th><th>Failed</th><th>Network errors</th><th>p95</th><th>p99</th></tr></thead><tbody>${Object.entries(
      summary.scenarios || {},
    )
      .map(
        ([name, stats]) =>
          `<tr><td>${escapeHTML(name)}</td><td>${stats.completed}</td><td>${stats.failed}</td><td>${stats.network_errors}</td><td>${stats.p95_ms.toFixed(2)} ms</td><td>${stats.p99_ms.toFixed(2)} ms</td></tr>`,
      )
      .join(
        "",
      )}</tbody></table><table aria-label="Recent requests"><thead><tr><th>Scenario</th><th>Status</th><th>Latency</th><th>Result</th><th>Bodies</th></tr></thead><tbody>${responses
      .slice(-100)
      .map(
        (response) =>
          `<tr><td>${escapeHTML(response.scenario)}</td><td>${response.status || "Network"}</td><td>${response.duration_ms.toFixed(2)} ms</td><td>${escapeHTML(response.error || (response.expected ? "Expected" : "Unexpected"))}</td><td><button class="text-button" type="button" data-inspect="${response.index - 1}">Inspect</button></td></tr>`,
      )
      .join("")}</tbody></table>`;
  $("#result-detail")
    .querySelectorAll("[data-inspect]")
    .forEach(
      (button) =>
        (button.onclick = () => {
          const response = responses[Number(button.dataset.inspect)];
          $("#sent-body").textContent = response.request_body
            ? pretty(response.request_body)
            : "Not recorded for this run.";
          $("#received-body").textContent =
            (response.body_truncated ? "[Response truncated]\n" : "") +
            (response.body !== undefined
              ? pretty(response.body)
              : response.body_text || response.error || "No response body.");
          $("#request-inspector").hidden = false;
          $("#request-inspector").scrollIntoView({ block: "nearest" });
        }),
    );
}
function parseIntervalSeconds(value) {
  return LoadLabUI.durationSeconds(value);
}
async function refreshStatus() {
  const status = await api("/api/status");
  $("#run-state").textContent = status.state.toUpperCase();
  if (
    status.result?.summary?.outcome &&
    !["running", "stopping"].includes(status.state)
  )
    $("#run-state").textContent +=
      ` / ${status.result.summary.outcome.toUpperCase()}`;
  const running = ["running", "stopping"].includes(status.state);
  runBusy = running;
  updateTraffic();
  $("#stop").disabled = !running || status.state === "stopping";
  if (!running && status.result) renderReport(status.result);
  if (status.error) notify(status.error, true);
  return running;
}
async function poll() {
  if (polling) return;
  polling = true;
  const generation = ++pollGeneration;
  let failures = 0;
  const tick = async () => {
    if (generation !== pollGeneration) return;
    try {
      const running = await refreshStatus();
      if (generation !== pollGeneration) return;
      if (failures > 0) notify("Connection restored.");
      failures = 0;
      $("#reconnect").hidden = true;
      if (running) {
        pollTimer = setTimeout(tick, 700);
        return;
      }
    } catch (error) {
      if (generation !== pollGeneration) return;
      failures++;
      $("#run-state").textContent = "DISCONNECTED";
      $("#reconnect").hidden = false;
      $("#stop").disabled = !runBusy;
      notify(
        "Connection interrupted. Retrying status; your setup is unchanged.",
        true,
      );
      pollTimer = setTimeout(
        tick,
        Math.min(500 * 2 ** Math.min(failures, 3), 5000),
      );
      return;
    }
    polling = false;
  };
  await tick();
}
$("#start").onclick = guard(async () => {
  if (!$("#authorized").checked)
    throw new Error(
      "Confirm that you are authorized to load test these targets.",
    );
  const submission = requestSubmission();
  $("#request-inspector").hidden = true;
  runBusy = true;
  $("#start").disabled = true;
  try {
    await api("/api/run", submission);
    notify("Load test started.");
    await poll();
  } catch (error) {
    runBusy = false;
    updateTraffic();
    throw error;
  }
});
$("#stop").onclick = guard(async () => {
  await api("/api/stop", {});
  await refreshStatus();
  notify("Scheduling stopped. Waiting for in-flight requests.");
});
$("#download-results").onclick = () => {
  if (lastReport) download(lastReport, "load-lab-results.json");
};
$("#choose-setup").onclick = () => $("#import-config").click();
$("#reconnect").onclick = guard(async () => {
  clearTimeout(pollTimer);
  pollGeneration++;
  polling = false;
  await poll();
});
render();
poll();

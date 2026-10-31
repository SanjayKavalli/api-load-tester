"use strict";
const byId = (id) => document.getElementById(id);
const labels = { normal: "Normal", burst: "Burst", multi_api_burst: "Multi-API Burst", weighted: "Weighted", coordinated: "Coordinated" };
const api = (name, method = "PATCH", weight = 0) => ({ name, method, weight, url: `https://demo.invalid/patients${method === "POST" ? "" : "/PT-00000001"}` });
const cases = {
  normal: { mode: "normal", users: 1, records: "fresh", apis: [api("create_patient", "POST", 100)], description: "A steady stream: one new patient request per illustrated arrival. 20 requests in this shortened demo." },
  burst: { mode: "burst", users: 100, records: "fresh", apis: [api("single_api_request", "POST", 100)], description: "100 users target one API in a burst. The demo illustrates one group; real Burst mode schedules groups at the configured rate." },
  multi_api_burst: { mode: "multi_api_burst", users: 100, records: "fresh", apis: [api("create_patient", "POST"), api("update_patient"), api("read_patient", "GET")], description: "100 users x 3 APIs = 300 requests per group, prepared before a shared start gate. Choose single or repeated bursts. There is no create-then-update chaining." },
  weighted: { mode: "weighted", users: 100, records: "mixed", apis: [api("create_patient", "POST", 60), api("update_patient", "PATCH", 40)], description: "Create 60%, update 40%. This illustrative sample uses exactly 60/40; real weighted selection is probabilistic and does not guarantee that split." },
  coordinated: { mode: "coordinated", users: 100, records: "fixed", apis: [api("actor_a_update"), api("actor_b_update")], description: "Every user runs both APIs with the same shared record: 100 x 2 = 200 requests. Roles execute independently, not as an ordered workflow." },
  "single-api-100-burst": { base: "burst", doc: "single-api-100-burst" },
  "weighted-create-update-60-40": { base: "weighted", doc: "weighted-create-update-60-40" },
  "100-users-one-record": { base: "coordinated", doc: "100-users-one-record" },
  "100-users-many-records": { base: "coordinated", records: "random", doc: "100-users-many-records", description: "100 users, two APIs per user, drawn from a shared dataset. Both actors in a user group get the same selected record. The demo uses a deterministic stand-in for random selection." },
  "coordinated-cycle-records": { base: "coordinated", records: "cycle", apis: [api("service_a_update"), api("service_b_update")], doc: "coordinated-cycle-records", description: "Cycle PT-00000001, PT-00000002, PT-00000003 across user groups. Both services in each group receive the same record." },
  "coordinated-three-roles-one-record": { base: "coordinated", apis: [api("role_a_update"), api("role_b_update"), api("role_c_read", "GET")], doc: "coordinated-three-roles-one-record", description: "100 users x 3 roles = 300 requests against one shared patient. Two independent writers and one reader; no ordering or response handoff." },
};
const dataset = [1, 2, 3].map((number) => ({ patient_id: `PT-${String(number).padStart(8, "0")}`, first_name: "Demo", last_name: `Patient${number}`, email: `patient${number}@example.test` }));
let current;
let timer;
let planned = [];
let sent = [];
let phase = "choose";
const json = (value) => JSON.stringify(value, null, 2);
function notice(text) {
  byId("feature-notice").textContent = text;
  byId("feature-notice").hidden = false;
}
function downloadJSON(value, filename) {
  const url = URL.createObjectURL(new Blob([json(value)], { type: "application/json" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function guide(next) {
  phase = next;
  document.querySelectorAll(".tour-target").forEach((element) => element.classList.remove("tour-target"));
  const instructions = {
    choose: [1, `Click ${labels[current.mode]} in Traffic profile to load this use case.`, `[data-mode="${current.mode}"]`],
    preview: [2, "Review the dummy APIs and dataset, then click Generate payload preview.", "#preview"],
    start: [3, "Inspect the generated JSON, then click Start demo to simulate this traffic option.", "#start"],
    running: [3, "Simulated run in progress. Stop ends the demo early; no network requests are sent.", "#stop"],
    inspect: [4, "Click a request number in the results to inspect its dummy request and response.", "#records button"],
    done: [4, "Tour complete. Inspect more requests, download the sample report, or choose another use case.", "#download"],
  };
  const [step, text, target] = instructions[next];
  byId("step-count").textContent = `STEP ${step} OF 4`;
  byId("instruction").textContent = text;
  document.querySelector(target)?.classList.add("tour-target");
}
function chart() {
  const canvas = byId("traffic-chart");
  const context = canvas.getContext("2d");
  context.clearRect(0, 0, canvas.width, canvas.height);
  const group = ["burst", "multi_api_burst", "coordinated"].includes(current.mode);
  const bars = group ? (byId("schedule").value === "repeat" && current.mode === "multi_api_burst" ? [2, 12] : [2]) : Array.from({ length: 20 }, (_, index) => index);
  context.fillStyle = "#16745c";
  bars.forEach((index) => context.fillRect(18 + index * 46, group ? 12 : 53, 20, group ? 65 : 24));
  context.fillStyle = "#52645e";
  context.font = "12px Segoe UI";
  context.fillText(group ? "Illustrative grouped arrivals" : "Illustrative steady arrivals", 18, 95);
  canvas.setAttribute("aria-label", `${group ? "Grouped" : "Steady"} simulated arrivals for ${labels[current.mode]}`);
}
function loadCase(key) {
  clearInterval(timer);
  const entry = cases[key] || cases.normal;
  current = { ...(entry.base ? cases[entry.base] : entry), ...entry };
  planned = [];
  sent = [];
  byId("use-case").value = cases[key] ? key : "normal";
  byId("schedule").value = "once";
  byId("schedule").disabled = false;
  byId("schedule-field").hidden = current.mode !== "multi_api_burst";
  byId("case-description").textContent = current.description;
  byId("users").textContent = current.users;
  byId("api-count").textContent = current.apis.length;
  byId("record-mode").textContent = { fresh: "Fresh generated IDs", mixed: "Fresh create / existing update", fixed: "One fixed record", cycle: "Cycle 3 records", random: "Random (illustrative)" }[current.records];
  byId("traffic").textContent = current.mode === "normal" ? "1 request / arrival" : current.mode === "weighted" ? "60% / 40%" : `${current.users * (current.mode === "burst" ? 1 : current.apis.length)} / group`;
  byId("apis").replaceChildren();
  current.apis.forEach((scenario) => {
    const row = document.createElement("tr");
    [scenario.name, `${scenario.method} ${scenario.url}`, scenario.weight ? `${scenario.weight}%` : "Every user"].forEach((text) => {
      const cell = document.createElement("td");
      cell.textContent = text;
      row.append(cell);
    });
    byId("apis").append(row);
  });
  byId("dataset").textContent = json(dataset);
  byId("rules").textContent = "Fresh IDs: PT-[0-9]{8} (Regex, Unique). Email: lower(body.first_name) + '.' + lower(body.last_name) + '@example.test' (Expression). Reuse existing / shared records skips Unique rules. This demo uses canned values, not the Go generator or persistent uniqueness history.";
  document.querySelectorAll("[data-mode]").forEach((button) => button.setAttribute("aria-pressed", "false"));
  byId("preview").disabled = true;
  byId("start").disabled = true;
  byId("stop").disabled = true;
  byId("download").disabled = true;
  byId("download-samples").disabled = true;
  byId("copy-preview").disabled = true;
  byId("feature-notice").hidden = true;
  byId("preview-note").textContent = "";
  byId("payload").textContent = "Select the highlighted traffic option to begin.";
  byId("request").textContent = "Select a result to inspect its request.";
  byId("response").textContent = "Select a result to inspect its response.";
  byId("run-state").textContent = "Ready";
  const doc = byId("scenario-doc");
  doc.hidden = !current.doc;
  if (current.doc) doc.href = `scenarios/${current.doc}.md`;
  renderResults();
  chart();
  guide("choose");
}
function bodyFor(scenario, user, sequence) {
  if (scenario.method === "GET") return null;
  const sharedIndex = current.records === "cycle" ? user % 3 : current.records === "random" ? (user * 7 + Math.floor(user / 3)) % 3 : 0;
  const body = { ...dataset[sharedIndex] };
  if (current.records === "fresh" || (current.records === "mixed" && scenario.method === "POST")) body.patient_id = `PT-${String(100000 + sequence).padStart(8, "0")}`;
  body.updated_by = scenario.name;
  return body;
}
function makeRecord(scenario, user, sequence, group) {
  const body = bodyFor(scenario, user, sequence);
  const patientId = body?.patient_id || dataset[current.records === "cycle" ? user % 3 : 0].patient_id;
  const conflict = scenario.method === "PATCH" && sequence % 37 === 0;
  const status = conflict ? 409 : scenario.method === "POST" ? 201 : 200;
  return {
    sequence, group, user: user + 1, api: scenario.name, status, latency_ms: 24 + (sequence * 17) % 153,
    request: { method: scenario.method, url: scenario.method === "POST" ? scenario.url : `https://demo.invalid/patients/${patientId}`, headers: { Accept: "application/json", ...(body ? { "Content-Type": "application/json" } : {}) }, body },
    response: { status, headers: { "Content-Type": "application/json", "X-Demo": "simulated" }, body: conflict ? { error: "illustrative_conflict", message: "Dummy concurrent-update conflict; not a real API result." } : { patient_id: patientId, result: scenario.method === "GET" ? "read" : scenario.method === "POST" ? "created" : "updated", simulated: true } },
  };
}
function buildRun() {
  const result = [];
  const groups = current.mode === "multi_api_burst" && byId("schedule").value === "repeat" ? 2 : 1;
  const users = current.mode === "normal" ? 20 : current.users;
  for (let group = 1; group <= groups; group++) {
    for (let user = 0; user < users; user++) {
      const selected = current.mode === "weighted" ? [current.apis[user % 10 < 6 ? 0 : 1]] : current.apis;
      selected.forEach((scenario) => result.push(makeRecord(scenario, user, result.length + 1, group)));
    }
  }
  return result;
}
function renderResults() {
  const failures = sent.filter((record) => record.status >= 400).length;
  byId("total").textContent = sent.length;
  byId("success").textContent = sent.length - failures;
  byId("failed").textContent = failures;
  const latencies = sent.map((record) => record.latency_ms).sort((left, right) => left - right);
  byId("p95").textContent = sent.length ? `${latencies[Math.ceil(sent.length * .95) - 1]} ms` : "--";
  byId("progress").value = planned.length ? sent.length / planned.length * 100 : 0;
  const rows = byId("records");
  rows.replaceChildren();
  if (!sent.length) {
    const row = rows.insertRow();
    const cell = row.insertCell();
    cell.colSpan = 4;
    cell.textContent = "No simulated requests yet.";
  }
  sent.slice(-30).forEach((record) => {
    const row = rows.insertRow();
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = `#${record.sequence}`;
    button.setAttribute("aria-label", `Inspect request ${record.sequence}`);
    button.addEventListener("click", () => {
      byId("request").textContent = json(record.request);
      byId("response").textContent = json(record.response);
      if (phase === "inspect" || phase === "done") guide("done");
    });
    row.insertCell().append(button);
    row.insertCell().textContent = record.api;
    const status = row.insertCell();
    status.textContent = record.status;
    if (record.status >= 400) status.classList.add("failed");
    row.insertCell().textContent = `${record.latency_ms} ms`;
  });
}
function finish(stopped = false) {
  clearInterval(timer);
  byId("run-state").textContent = stopped ? "Stopped (simulated)" : "Completed (simulated)";
  byId("stop").disabled = true;
  byId("start").disabled = false;
  byId("download").disabled = !sent.length;
  byId("preview").disabled = false;
  byId("schedule").disabled = false;
  guide(sent.length ? "inspect" : "start");
}
document.querySelectorAll("[data-mode]").forEach((button) => button.addEventListener("click", () => {
  if (button.dataset.mode !== current.mode) {
    history.replaceState(null, "", `#${button.dataset.mode}`);
    loadCase(button.dataset.mode);
  }
  if (phase === "running") return;
  button.setAttribute("aria-pressed", "true");
  byId("preview").disabled = false;
  guide("preview");
}));
byId("use-case").addEventListener("change", () => { location.hash = byId("use-case").value; });
byId("restart").addEventListener("click", () => loadCase(byId("use-case").value));
byId("schedule").addEventListener("change", chart);
byId("preview").addEventListener("click", () => {
  byId("payload").textContent = json(current.apis.map((scenario, index) => makeRecord(scenario, 0, index + 1, 1).request));
  byId("preview-note").textContent = "One dummy request per API. GET requests have no JSON body.";
  byId("start").disabled = false;
  byId("download-samples").disabled = false;
  byId("copy-preview").disabled = false;
  guide("start");
});
byId("start").addEventListener("click", () => {
  planned = buildRun();
  sent = [];
  byId("start").disabled = true;
  byId("stop").disabled = false;
  byId("preview").disabled = true;
  byId("download").disabled = true;
  byId("schedule").disabled = true;
  byId("run-state").textContent = "Running (simulated)";
  byId("request").textContent = "Select a result to inspect its request.";
  byId("response").textContent = "Select a result to inspect its response.";
  guide("running");
  renderResults();
  let tick = 0;
  timer = setInterval(() => {
    tick++;
    const target = Math.min(planned.length, Math.ceil(planned.length * tick / 10));
    sent = planned.slice(0, target);
    renderResults();
    if (sent.length === planned.length) finish();
  }, 180);
});
byId("stop").addEventListener("click", () => finish(true));
byId("download").addEventListener("click", () => {
  const report = { simulated: true, notice: "Dummy requests and responses. Not real performance measurements.", use_case: byId("use-case").value, mode: current.mode, schedule: current.mode === "multi_api_burst" ? byId("schedule").value : "illustrative sample", outcome: byId("run-state").textContent, total: sent.length, successful: sent.filter((record) => record.status < 400).length, failed: sent.filter((record) => record.status >= 400).length, records: sent };
  downloadJSON(report, `load-lab-demo-${byId("use-case").value}.json`);
});
document.querySelectorAll("[data-info]").forEach((button, index) => {
  const wrap = document.createElement("span");
  wrap.className = "tip-wrap";
  const tip = document.createElement("span");
  tip.className = "tooltip";
  tip.id = `feature-tip-${index}`;
  tip.setAttribute("role", "tooltip");
  tip.textContent = button.dataset.info;
  button.setAttribute("aria-describedby", tip.id);
  button.title = button.dataset.info;
  button.replaceWith(wrap);
  wrap.append(button, tip);
  if (button.getAttribute("aria-disabled") === "true") button.addEventListener("click", () => notice(button.dataset.info));
});
byId("export-setup").addEventListener("click", () => {
  downloadJSON({ format: "load-lab-demo-setup", version: 1, simulated: true, use_case: byId("use-case").value, burst_schedule: current.mode === "multi_api_burst" ? byId("schedule").value : "once" }, "load-lab-demo-setup.json");
  notice("Demo setup exported: selected use case and burst schedule only. Import it here to restore the tour; use the local app for real Load Lab setups.");
});
byId("import-setup").addEventListener("click", () => byId("setup-file").click());
byId("setup-file").addEventListener("change", async () => {
  const file = byId("setup-file").files[0];
  byId("setup-file").value = "";
  if (!file) return;
  try {
    if (file.size > 16384) throw new Error("Demo setups must be 16 KiB or smaller.");
    const setup = JSON.parse(await file.text());
    if (!setup || setup.format !== "load-lab-demo-setup" || setup.version !== 1 || setup.simulated !== true || !Object.hasOwn(cases, setup.use_case) || !["once", "repeat"].includes(setup.burst_schedule)) throw new Error("Choose a demo setup exported from this tour. Real Load Lab setups and CLI configs require the local application.");
    if (setup.burst_schedule === "repeat" && setup.use_case !== "multi_api_burst") throw new Error("Repeated bursts require the Multi-API Burst demo.");
    history.replaceState(null, "", `#${setup.use_case}`);
    loadCase(setup.use_case);
    byId("schedule").value = setup.burst_schedule;
    chart();
    notice("Demo setup imported. Click the highlighted traffic option to continue. No real payloads or credentials were imported.");
  } catch (error) {
    notice(`Import failed; current tour unchanged. ${error.message}`);
  }
});
byId("download-samples").addEventListener("click", () => downloadJSON({ simulated: true, samples: JSON.parse(byId("payload").textContent) }, "load-lab-demo-samples.json"));
byId("copy-preview").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(byId("payload").textContent);
    notice("Dummy payload copied.");
  } catch {
    const range = document.createRange();
    range.selectNodeContents(byId("payload"));
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
    byId("payload").focus();
    notice("Clipboard unavailable. The dummy payload is selected; copy it with your browser's copy command.");
  }
});
window.addEventListener("hashchange", () => loadCase(location.hash.slice(1)));
loadCase(location.hash.slice(1));
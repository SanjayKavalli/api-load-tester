"use strict";
(() => {
  const $ = (selector) => document.querySelector(selector);
  const pretty = (value) => JSON.stringify(value, null, 2);
  let snapshot,
    rows = [];
  function option(select, value, text) {
    const item = document.createElement("option");
    item.value = value;
    item.textContent = text;
    select.append(item);
  }
  function table(headers, values) {
    const element = document.createElement("table"),
      head = element.createTHead().insertRow();
    headers.forEach((text) => {
      const cell = document.createElement("th");
      cell.textContent = text;
      head.append(cell);
    });
    const body = element.createTBody();
    values.forEach((values) => {
      const row = body.insertRow();
      values.forEach((text) => {
        const cell = row.insertCell();
        cell.textContent = String(text ?? "");
      });
    });
    $("#analysis-table").replaceChildren(element);
  }
  function update() {
    if (!snapshot) return;
    const selected = $("#analysis-scenario").value,
      filter = $("#analysis-filter").value.toLowerCase();
    const previous = $("#analysis-item").value;
    $("#analysis-item").replaceChildren();
    $("#analysis-response-panel").hidden = snapshot.kind !== "results";
    $("#analysis-item-label").hidden = snapshot.kind === "rules";
    $("#analysis-filter-label").hidden = snapshot.kind === "preview";
    if (snapshot.kind === "preview") {
      const value = snapshot.data.bodies[selected];
      rows = Array.isArray(value) ? value : [value];
      rows.forEach((value, index) =>
        option($("#analysis-item"), index, `Sample ${index + 1}`),
      );
      $("#analysis-table").replaceChildren();
    } else if (snapshot.kind === "rules") {
      const scenario = snapshot.data.find((item) => item.name === selected);
      const rules = scenario.rules.map((rule, index) => [
        index + 1,
        rule.path.join("."),
        rule.regex ? "Regex" : "Expression",
        rule.regex || rule.expression,
        rule.unique ? "Yes" : "No",
      ]);
      table(
        ["Order", "Field", "Source", "Pattern / expression", "Unique"],
        rules.filter((row) => row.join(" ").toLowerCase().includes(filter)),
      );
      rows = [scenario.body];
      $("#analysis-body-title").textContent = "Payload template";
    } else {
      rows = snapshot.data.responses.filter(
        (item) =>
          (selected === "*" || item.scenario === selected) &&
          `${item.scenario} ${item.status} ${item.error || ""}`
            .toLowerCase()
            .includes(filter),
      );
      rows.forEach((item, index) =>
        option(
          $("#analysis-item"),
          index,
          `#${item.index} / ${item.scenario} / ${item.status || "Network"}`,
        ),
      );
      table(
        ["API", "Completed", "Failed", "p95 ms", "p99 ms"],
        Object.entries(snapshot.data.summary.scenarios || {})
          .filter(([name]) => selected === "*" || name === selected)
          .map(([name, value]) => [
            name,
            value.completed,
            value.failed,
            value.p95_ms.toFixed(2),
            value.p99_ms.toFixed(2),
          ]),
      );
    }
    if (
      [...$("#analysis-item").options].some((item) => item.value === previous)
    )
      $("#analysis-item").value = previous;
    showBody();
  }
  function showBody() {
    const item = rows[Number($("#analysis-item").value) || 0];
    if (snapshot.kind === "results") {
      $("#analysis-primary").textContent = item?.request_body
        ? pretty(item.request_body)
        : "Request body was not recorded.";
      $("#analysis-response").textContent = item
        ? item.body !== undefined
          ? pretty(item.body)
          : item.body_text || item.error || "No response body."
        : "No matching request.";
      $("#analysis-body-title").textContent = "Sent request";
      $("#analysis-response-title").textContent = item?.body_truncated
        ? "Response JSON (truncated)"
        : "Response JSON";
    } else $("#analysis-primary").textContent = pretty(item ?? {});
  }
  $("#analysis-scenario").onchange = update;
  $("#analysis-filter").oninput = update;
  $("#analysis-item").onchange = showBody;
  $("#analysis-copy").onclick = async () => {
    try {
      await navigator.clipboard.writeText($("#analysis-primary").textContent);
    } catch {
      $("#analysis-status").textContent =
        "Clipboard unavailable. Select the JSON to copy it.";
    }
  };
  $("#analysis-download").onclick = () => {
    const url = URL.createObjectURL(
      new Blob([pretty(snapshot)], { type: "application/json" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = `load-lab-${snapshot.kind}-snapshot.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };
  const id = location.hash.slice(1);
  if (!/^[0-9a-f-]{36}$/i.test(id)) {
    $("#analysis-status").textContent =
      "Open this view using an analysis icon in the workspace.";
    return;
  }
  const channel = new BroadcastChannel(`load-lab-analysis-${id}`);
  const timeout = setTimeout(() => {
    $("#analysis-status").textContent =
      "Snapshot unavailable. Reopen this view from the workspace.";
    channel.close();
  }, 10000);
  channel.onmessage = (event) => {
    if (snapshot || !["preview", "rules", "results"].includes(event.data?.kind))
      return;
    snapshot = event.data;
    clearTimeout(timeout);
    channel.close();
    const title = {
      preview: "Payload preview",
      rules: "Field rules",
      results: "Run results",
    }[snapshot.kind];
    document.title = `Load Lab | ${title}`;
    $("#analysis-title").textContent = title;
    $("#analysis-status").textContent =
      `${snapshot.kind === "preview" && !snapshot.data.generated ? "Template snapshot" : "Read-only snapshot"} / ${new Date(snapshot.created_at).toLocaleString()}${snapshot.kind === "results" ? ` / latest ${snapshot.data.responses.length} recorded requests` : ""}`;
    $("#analysis-scenario").replaceChildren();
    if (snapshot.kind === "results") {
      option($("#analysis-scenario"), "*", "All APIs");
      Object.keys(snapshot.data.summary.scenarios || {}).forEach((name) =>
        option($("#analysis-scenario"), name, name),
      );
    } else if (snapshot.kind === "rules")
      snapshot.data.forEach((item) =>
        option($("#analysis-scenario"), item.name, item.name),
      );
    else
      Object.keys(snapshot.data.bodies).forEach((name) =>
        option($("#analysis-scenario"), name, name),
      );
    $("#analysis-copy").disabled = false;
    $("#analysis-download").disabled = false;
    update();
  };
  channel.postMessage({ type: "ready" });
})();

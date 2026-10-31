"use strict";
document.querySelectorAll("[data-analysis]").forEach((button) => {
  button.onclick = guard(() => {
    const kind = button.dataset.analysis;
    let data;
    if (kind === "results") {
      if (!lastReport)
        throw new Error("Finish a run before opening result analysis.");
      data = { ...lastReport, responses: lastReport.responses.slice(-100) };
    } else if (kind === "preview") {
      data = {
        generated: Boolean(previewBodies),
        bodies:
          previewBodies ||
          Object.fromEntries(
            scenarios.map((scenario) => [scenario.name, scenario.body]),
          ),
      };
    } else {
      data = scenarios.map((scenario) => ({
        name: scenario.name,
        body: scenario.body,
        rules: scenario.rules,
      }));
    }
    const id = crypto.randomUUID();
    const snapshotData = structuredClone(data);
    const channel = new BroadcastChannel(`load-lab-analysis-${id}`);
    let connected = false;
    channel.onmessage = (event) => {
      if (event.data?.type !== "ready") return;
      connected = true;
      channel.postMessage({
        kind,
        created_at: new Date().toISOString(),
        data: snapshotData,
      });
    };
    window.open(`/analysis.html#${id}`, "_blank", "noopener");
    setTimeout(() => {
      if (!connected)
        notify(
          "The analysis tab did not connect. Allow popups and try again.",
          true,
        );
    }, 10000);
    setTimeout(() => channel.close(), 60000);
  });
});

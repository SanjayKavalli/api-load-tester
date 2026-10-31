"use strict";
((root) => {
  const object = (value) =>
    Boolean(value) && typeof value === "object" && !Array.isArray(value);
  const integer = (value, minimum, maximum, name) => {
    if (!Number.isInteger(value) || value < minimum || value > maximum)
      throw new Error(
        `${name} must be a whole number between ${minimum} and ${maximum}.`,
      );
    return value;
  };
  function safeNumbers(value, depth = 0) {
    if (depth > 100) throw new Error("JSON nesting exceeds 100 levels.");
    if (typeof value === "number" && !Number.isFinite(value))
      throw new Error(
        "JSON numbers must be finite. Encode large identifiers as strings.",
      );
    if (
      typeof value === "number" &&
      Number.isInteger(value) &&
      !Number.isSafeInteger(value)
    )
      throw new Error(
        "An integer exceeds safe JSON precision. Encode large identifiers as strings.",
      );
    if (value && typeof value === "object")
      Object.values(value).forEach((child) => safeNumbers(child, depth + 1));
    return value;
  }
  function parseJSON(text) {
    return safeNumbers(JSON.parse(text));
  }
  function durationSeconds(value) {
    if (typeof value !== "string")
      throw new Error("Duration must be text such as 30s or 1m30s.");
    const text = value.trim();
    const pattern = /(\d+(?:\.\d*)?|\.\d+)(ns|us|\u00b5s|\u03bcs|ms|s|m|h)/gy;
    const scale = {
      ns: 1e-9,
      us: 1e-6,
      "\u00b5s": 1e-6,
      "\u03bcs": 1e-6,
      ms: 0.001,
      s: 1,
      m: 60,
      h: 3600,
    };
    let position = 0,
      seconds = 0,
      match;
    while ((match = pattern.exec(text))) {
      seconds += Number(match[1]) * scale[match[2]];
      position = pattern.lastIndex;
    }
    if (!text || position !== text.length || !Number.isFinite(seconds))
      throw new Error(
        `Invalid duration: ${value}. Use values such as 30s or 1m30s.`,
      );
    return seconds;
  }
  function dataset(value) {
    if (
      !Array.isArray(value) ||
      !value.length ||
      value.some((record) => !object(record))
    )
      throw new Error("Dataset must be a non-empty JSON array of objects.");
    return safeNumbers(value);
  }
  function rules(value, body) {
    if (!Array.isArray(value)) throw new Error("Field rules must be an array.");
    value.forEach((rule, index) => {
      if (
        !object(rule) ||
        !Array.isArray(rule.path) ||
        !rule.path.length ||
        rule.path.some((part) => typeof part !== "string")
      )
        throw new Error("Each field rule needs a non-empty string path.");
      if (
        (typeof rule.regex === "string" && rule.regex.length > 0) ===
        (typeof rule.expression === "string" && rule.expression.length > 0)
      )
        throw new Error(
          "Each field rule needs exactly one regex or expression.",
        );
      if (rule.unique !== undefined && typeof rule.unique !== "boolean")
        throw new Error("Unique must be true or false.");
      for (const earlier of value.slice(0, index)) {
        const size = Math.min(earlier.path.length, rule.path.length);
        if (
          earlier.path
            .slice(0, size)
            .every((part, position) => part === rule.path[position])
        )
          throw new Error(
            "Duplicate or overlapping field rules are not allowed.",
          );
      }
      let parent = body;
      for (const part of rule.path) {
        if (
          !parent ||
          typeof parent !== "object" ||
          !Object.hasOwn(parent, part)
        )
          throw new Error(
            `Field rule path does not exist: ${rule.path.join(".")}`,
          );
        parent = parent[part];
      }
    });
    return value;
  }
  function validateSetup(input) {
    if (
      !object(input) ||
      !object(input.config) ||
      !object(input.files) ||
      !Array.isArray(input.config.scenarios) ||
      !input.config.scenarios.length
    )
      throw new Error("Choose an exported Load Lab setup (config + files).");
    if (input.version !== undefined && input.version !== 1)
      throw new Error("Unsupported setup version.");
    safeNumbers(input);
    const config = input.config;
    const mode = config.execution_mode || "weighted";
    if (!["weighted", "coordinated", "multi_api_burst"].includes(mode))
      throw new Error("Unknown execution mode.");
    const profile =
      input.profile ||
      (mode !== "weighted"
        ? mode
        : config.scenarios.length > 1
          ? "weighted"
          : config.burst_size > 1
            ? "burst"
            : "normal");
    if (
      ![
        "normal",
        "burst",
        "weighted",
        "coordinated",
        "multi_api_burst",
      ].includes(profile) ||
      (["coordinated", "multi_api_burst"].includes(profile)
        ? profile
        : "weighted") !== mode
    )
      throw new Error("Setup profile does not match its execution mode.");
    const users = integer(config.burst_size ?? 1, 1, 100000, "Users per burst");
    const maximum = integer(
      config.max_inflight ?? 10,
      1,
      mode === "multi_api_burst" ? 10000 : 1000,
      "Max in flight",
    );
    integer(config.rate ?? 1, 1, 10000, "Bursts per second");
    const duration = durationSeconds(config.duration || "30s"),
      timeout = durationSeconds(config.timeout || "10s");
    if (duration <= 0 || duration > 3600 || timeout <= 0 || timeout > 60)
      throw new Error(
        "Duration must be positive and at most 1h; timeout must be positive and at most 1m.",
      );
    if (!["once", "repeat"].includes(config.burst_mode || "once"))
      throw new Error("Invalid burst schedule.");
    if (durationSeconds(config.burst_interval || "1s") < 0.001)
      throw new Error("Burst interval must be at least 1ms.");
    if (!["fresh", "existing"].includes(config.data_mode || "fresh"))
      throw new Error("Invalid data strategy.");
    if (
      !["fixed", "cycle", "random"].includes(
        config.shared_dataset_mode || "fixed",
      )
    )
      throw new Error("Invalid shared record mode.");
    if (
      config.record_request_bodies !== undefined &&
      typeof config.record_request_bodies !== "boolean"
    )
      throw new Error("Record request bodies must be true or false.");
    if (config.unique_history_file)
      throw new Error("The web runner manages the uniqueness history path.");
    const names = new Set();
    config.scenarios.forEach((item) => {
      if (
        !object(item) ||
        typeof item.name !== "string" ||
        !item.name.trim() ||
        names.has(item.name.trim())
      )
        throw new Error("Each scenario needs a unique name.");
      names.add(item.name.trim());
      integer(item.weight, 1, 100, "API percentage");
      let url;
      try {
        url = new URL(item.url);
      } catch {
        throw new Error("Each API needs a valid HTTP(S) URL.");
      }
      if (!["http:", "https:"].includes(url.protocol))
        throw new Error("Each API needs a valid HTTP(S) URL.");
      if (
        !["POST", "PUT", "PATCH", "GET", "DELETE", "HEAD", "OPTIONS"].includes(
          item.method || "POST",
        )
      )
        throw new Error("Unsupported HTTP method.");
      if (
        !Object.hasOwn(input.files, item.body_file) ||
        !object(input.files[item.body_file])
      )
        throw new Error(`Missing or invalid request body: ${item.body_file}`);
      if (
        item.headers !== undefined &&
        (!object(item.headers) ||
          Object.values(item.headers).some(
            (value) => typeof value !== "string",
          ))
      )
        throw new Error("Headers must be a JSON object with string values.");
      if (
        item.expected_statuses !== undefined &&
        (!Array.isArray(item.expected_statuses) ||
          item.expected_statuses.some(
            (value) => !Number.isInteger(value) || value < 100 || value > 599,
          ))
      )
        throw new Error("Expected statuses must be HTTP codes.");
      if (
        item.overlay_dataset_keys !== undefined &&
        typeof item.overlay_dataset_keys !== "boolean"
      )
        throw new Error("Dataset overlay must be true or false.");
      if (item.dataset_mode && !["cycle", "random"].includes(item.dataset_mode))
        throw new Error("Invalid scenario record mode.");
      if (item.dataset_file) dataset(input.files[item.dataset_file]);
      rules(item.field_rules || [], input.files[item.body_file]);
    });
    if (
      mode === "weighted" &&
      config.scenarios.reduce((sum, item) => sum + item.weight, 0) !== 100
    )
      throw new Error("API percentages must total 100%.");
    if (mode === "multi_api_burst" && users * config.scenarios.length > maximum)
      throw new Error("Max in flight must cover the complete multi-API burst.");
    if (config.shared_dataset_file)
      dataset(input.files[config.shared_dataset_file]);
    if (
      new Blob([JSON.stringify({ config, files: input.files })]).size >
      8 * 1024 * 1024
    )
      throw new Error("The complete setup exceeds the 8 MiB submission limit.");
    return { ...input, profile, version: 1 };
  }
  const api = {
    safeNumbers,
    parseJSON,
    durationSeconds,
    dataset,
    rules,
    validateSetup,
  };
  root.LoadLabUI = api;
  if (typeof module !== "undefined") module.exports = api;
})(globalThis);

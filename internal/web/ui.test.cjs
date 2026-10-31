const test = require("node:test");
const assert = require("node:assert/strict");
const ui = require("./static/ui-core.js");
const setup = () => ({
  config: {
    duration: "30s",
    rate: 1,
    burst_size: 1,
    max_inflight: 10,
    timeout: "10s",
    scenarios: [
      {
        name: "Test",
        weight: 100,
        url: "http://127.0.0.1/test",
        body_file: "body.json",
        field_rules: [],
      },
    ],
  },
  files: { "body.json": { id: "TEST" } },
  profile: "normal",
});
test("compound intervals are preserved", () => {
  assert.equal(ui.durationSeconds("1m30s"), 90);
  assert.equal(ui.durationSeconds("250ms"), 0.25);
  assert.throws(() => ui.durationSeconds("nonsense"));
});
test("unsafe integer identifiers are rejected", () => {
  assert.throws(() => ui.parseJSON('{"id":1e400}'));
  assert.throws(() => ui.parseJSON('{"id":9007199254740993}'));
  assert.deepEqual(ui.parseJSON('{"id":"9007199254740993"}'), {
    id: "9007199254740993",
  });
});
test("malformed imports are validated without mutation", () => {
  const value = setup();
  value.config.scenarios[0].field_rules = {};
  const before = JSON.stringify(value);
  assert.throws(() => ui.validateSetup(value));
  assert.equal(JSON.stringify(value), before);
});
test("overlapping rules and zero users are rejected", () => {
  const value = setup();
  value.config.burst_size = 0;
  assert.throws(() => ui.validateSetup(value));
  assert.throws(() =>
    ui.rules(
      [
        { path: ["id"], regex: "A", unique: true },
        { path: ["id"], expression: '"B"' },
      ],
      { id: "" },
    ),
  );
});
test("large valid setup bundles can round trip", () => {
  const value = setup();
  value.files["body.json"] = { data: "x".repeat(3.1 * 1024 * 1024) };
  value.files["data.json"] = [{ data: "x".repeat(3.1 * 1024 * 1024) }];
  value.config.scenarios[0].dataset_file = "data.json";
  assert.equal(ui.validateSetup(value).version, 1);
});

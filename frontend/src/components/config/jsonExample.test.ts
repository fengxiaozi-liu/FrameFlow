import { describe, expect, it } from "vitest";
import { jsonFields } from "./jsonExample";

describe("JSON example field preview", () => {
  it("parses nested fields and escapes pointer tokens", () => {
    const result = jsonFields(
      '{"data":[{"url":"example","duration":5}],"a/b":{"~key":true}}',
    );
    expect(result.error).toBe("");
    expect(result.fields).toContainEqual({
      pointer: "/data/0/duration",
      type: "number",
    });
    expect(result.fields).toContainEqual({
      pointer: "/a~1b/~0key",
      type: "boolean",
    });
  });
  it("reports invalid and oversized examples without throwing", () => {
    expect(jsonFields("{bad").error).not.toBe("");
    expect(jsonFields("x".repeat(131073)).fields).toEqual([]);
  });
});

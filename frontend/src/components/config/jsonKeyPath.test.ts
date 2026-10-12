import { describe, expect, it } from "vitest";
import { keyToPointer, pointerToKey } from "./jsonKeyPath";

describe("JSON key path display", () => {
  it("converts simple keys without exposing pointer syntax", () => {
    expect(pointerToKey("/data")).toBe("data");
    expect(keyToPointer("data")).toBe("/data");
  });
  it("supports nested keys and escaped JSON property names", () => {
    expect(pointerToKey("/result/models")).toBe("result.models");
    expect(keyToPointer("result.models")).toBe("/result/models");
    expect(keyToPointer("items.a~b")).toBe("/items/a~0b");
  });
  it("keeps old pointer values readable while editing", () => {
    expect(keyToPointer("/items/0/id")).toBe("/items/0/id");
    expect(keyToPointer("/a~1b/~0id")).toBe("/a~1b/~0id");
  });
  it("preserves literal dots, slashes and partial nested input", () => {
    const pointer = "/input/a.b/a~1b/~0name";
    expect(pointerToKey(pointer)).toBe("input.a\\.b.a\\/b.~name");
    expect(keyToPointer(pointerToKey(pointer))).toBe(pointer);
    expect(pointerToKey(keyToPointer("input."))).toBe("input.");
  });
});

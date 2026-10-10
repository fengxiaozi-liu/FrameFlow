import { describe, expect, it } from "vitest";
import { defaultEntityName } from "./entityDefaults";

describe("default entity names", () => {
  it("numbers characters and locations independently and skips occupied names", () => {
    const entities = [
      { kind: "character" as const, name: "人物 1" },
      { kind: "character" as const, name: "人物 2" },
      { kind: "character" as const, name: "林夏" },
      { kind: "location" as const, name: "场景 1" },
    ];
    expect(defaultEntityName("character", entities)).toBe("人物 3");
    expect(defaultEntityName("location", entities)).toBe("场景 2");
    expect(defaultEntityName("character", [])).toBe("人物 1");
    expect(entities[2]!.name).toBe("林夏");
  });
});

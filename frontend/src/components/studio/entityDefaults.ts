import type { EntityVersion } from "../../api/bindings";

/** Suggest a free project name only when opening a new entity form. */
export function defaultEntityName(
  kind: "location" | "character",
  entities: Pick<EntityVersion, "kind" | "name">[],
): string {
  const prefix = kind === "location" ? "场景" : "人物";
  const names = new Set(
    entities
      .filter((entity) => entity.kind === kind)
      .map((entity) => entity.name.trim()),
  );
  let index = 1;
  while (names.has(`${prefix} ${index}`)) index++;
  return `${prefix} ${index}`;
}

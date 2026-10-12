import type { JSONField } from "./jsonExample";

export interface BusinessField {
  id: string;
  label: string;
  required?: boolean;
}
export type ContractDirection = "request" | "response";

export interface JSONFieldNode extends JSONField {
  key: string;
  children: JSONFieldNode[];
}

// jsonFields visits parents before children. Keep pointers as identity while
// showing only the local key inside its actual parent group.
export function jsonFieldTree(fields: JSONField[]): JSONFieldNode[] {
  const roots: JSONFieldNode[] = [];
  const nodes = new Map<string, JSONFieldNode>();
  for (const field of fields) {
    const separator = field.pointer.lastIndexOf("/");
    const parent = nodes.get(field.pointer.slice(0, separator));
    const key = field.pointer
      .slice(separator + 1)
      .replaceAll("~1", "/")
      .replaceAll("~0", "~");
    const node: JSONFieldNode = {
      ...field,
      key: parent?.type === "array" ? `[${key}]` : key || '""',
      children: [],
    };
    nodes.set(field.pointer, node);
    (parent?.children || roots).push(node);
  }
  return roots;
}

export function bindingType(id: string, direction: ContractDirection): string {
  if (direction === "response") return "string";
  if (id === "images" || id === "messages") return "array";
  return id === "duration" ? "number" : "string";
}

export function bindingUnavailable(
  field: JSONField,
  business: BusinessField,
  bindings: Record<string, string>,
  direction: ContractDirection,
): string {
  if (field.type !== bindingType(business.id, direction))
    return `需要 ${bindingType(business.id, direction)} 类型`;
  if (bindings[business.id] && bindings[business.id] !== field.pointer)
    return "已绑定其他字段，请先解除";
  if (
    direction === "request" &&
    Object.values(bindings).some(
      (path) =>
        path !== field.pointer &&
        (path.startsWith(`${field.pointer}/`) ||
          field.pointer.startsWith(`${path}/`)),
    )
  )
    return "父级或子级已有绑定，请先解除";
  return "";
}

function pointerKeys(pointer: string): string[] {
  return pointer
    .slice(1)
    .split("/")
    .map((key) => key.replaceAll("~1", "/").replaceAll("~0", "~"));
}

export function exampleValue(root: unknown, pointer: string): unknown {
  return pointerKeys(pointer).reduce<unknown>((value, key) => {
    if (value === null || typeof value !== "object") return undefined;
    return Object.hasOwn(value, key)
      ? (value as Record<string, unknown>)[key]
      : undefined;
  }, root);
}

// Only existing primitive fields are edited here; structure changes use the JSON editor.
export function updateExampleValue(
  example: string,
  pointer: string,
  value: string | number | boolean,
): string {
  const root: unknown = JSON.parse(example);
  const keys = pointerKeys(pointer);
  const leaf = keys.pop()!;
  let parent = root;
  for (const key of keys) {
    if (
      parent === null ||
      typeof parent !== "object" ||
      !Object.hasOwn(parent, key)
    )
      throw new Error("字段已不存在，请重新解析 JSON");
    parent = (parent as Record<string, unknown>)[key];
  }
  if (
    parent === null ||
    typeof parent !== "object" ||
    !Object.hasOwn(parent, leaf)
  )
    throw new Error("字段已不存在，请重新解析 JSON");
  (parent as Record<string, unknown>)[leaf] = value;
  return JSON.stringify(root, null, 2);
}

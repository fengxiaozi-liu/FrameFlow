export interface JSONField {
  pointer: string;
  type: string;
}

export function jsonFields(example: string): {
  fields: JSONField[];
  error: string;
} {
  if (!example.trim()) return { fields: [], error: "请粘贴 JSON 示例" };
  if (example.length > 131072)
    return { fields: [], error: "JSON 示例不能超过 128 KB" };
  try {
    const root: unknown = JSON.parse(example);
    const fields: JSONField[] = [];
    function visit(value: unknown, pointer: string, depth: number) {
      if (depth > 32 || fields.length > 4096)
        throw new Error("JSON 示例过深或字段过多");
      if (pointer)
        fields.push({
          pointer,
          type: Array.isArray(value)
            ? "array"
            : value === null
              ? "null"
              : typeof value,
        });
      if (Array.isArray(value))
        value.forEach((item, index) =>
          visit(item, `${pointer}/${index}`, depth + 1),
        );
      else if (value !== null && typeof value === "object") {
        for (const [key, item] of Object.entries(value)) {
          visit(
            item,
            `${pointer}/${key.replaceAll("~", "~0").replaceAll("/", "~1")}`,
            depth + 1,
          );
        }
      }
    }
    visit(root, "", 0);
    return { fields, error: "" };
  } catch (error) {
    return {
      fields: [],
      error:
        error instanceof SyntaxError
          ? "JSON 格式无效，请检查引号和逗号"
          : (error as Error).message,
    };
  }
}

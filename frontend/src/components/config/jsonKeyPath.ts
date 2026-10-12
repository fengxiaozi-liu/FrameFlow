function decodeToken(token: string): string {
  return token.replaceAll("~1", "/").replaceAll("~0", "~");
}

function encodeToken(token: string): string {
  return token.replaceAll("~", "~0").replaceAll("/", "~1");
}

/** Display JSON Pointer values as keys while keeping the API contract intact. */
export function pointerToKey(pointer: string | undefined): string {
  if (!pointer) return "";
  if (!pointer.startsWith("/")) return pointer;
  return pointer
    .slice(1)
    .split("/")
    .map((token) =>
      decodeToken(token).replace(/[\\./]/g, (character) => `\\${character}`),
    )
    .join(".");
}

/** Accept a key or dotted key path and convert it to a JSON Pointer. */
export function keyToPointer(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return "";
  // Existing encoded pointers must remain intact, including ~0/~1 escapes.
  if (trimmed.startsWith("/")) return trimmed;
  const path: string[] = [];
  let token = "";
  for (let index = 0; index < trimmed.length; index++) {
    const character = trimmed[index]!;
    if (character === "\\" && index + 1 < trimmed.length) {
      token += trimmed[++index];
    } else if (character === ".") {
      path.push(token);
      token = "";
    } else {
      token += character;
    }
  }
  path.push(token);
  return `/${path.map(encodeToken).join("/")}`;
}

export function baseAddressError(value: string): string {
  try {
    const url = new URL(value.trim());
    if (
      url.protocol === "https:" &&
      !url.username &&
      !url.password &&
      !url.search &&
      !url.hash &&
      url.pathname === "/" &&
      !/[?#\\]/.test(value) &&
      /^https:\/\/[^/]+\/?$/.test(value.trim())
    )
      return "";
  } catch {
    /* The same hint covers incomplete input while editing. */
  }
  return "请填写 HTTPS 基础地址（域名和可选端口），不要包含接口路径、账号、查询参数或片段。";
}

export function requestAddress(baseURL: string, path?: string): string {
  if (!baseURL || baseAddressError(baseURL))
    return "请先配置有效的厂商基础 URL";
  if (
    !path ||
    !path.startsWith("/") ||
    path.startsWith("//") ||
    /[\\#\s]/.test(path)
  )
    return "请填写从 / 开始的有效请求路径";
  return new URL(baseURL.trim()).origin + path;
}

import { describe, expect, it } from "vitest";
import { baseAddressError, requestAddress } from "./connectionAddress";
describe("connection origins and address previews", () => {
  it("accepts origins and ports but rejects endpoint URLs and credentials", () => {
    for (const base of ["https://example.com", "https://example.com:8443/"])
      expect(baseAddressError(base)).toBe("");
    for (const base of [
      "http://example.com",
      "https://example.com/v1",
      "https://user:pass@example.com",
      "https://example.com/?key=x",
      "https://example.com/#x",
      "https://example.com/?",
      "https://example.com\\api",
    ])
      expect(baseAddressError(base)).not.toBe("");
  });
  it("shows the actual origin and complete path including task placeholders", () => {
    expect(
      requestAddress("https://example.com:8443/", "/tasks/{task_id}"),
    ).toBe("https://example.com:8443/tasks/{task_id}");
    expect(requestAddress("https://example.com", "/v1/generate")).toBe(
      "https://example.com/v1/generate",
    );
    expect(
      requestAddress("https://example.com", "//other.example/path"),
    ).not.toContain("https://other");
  });
});

import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ProgressBar from "./ProgressBar.vue";
import StatusBadge from "./StatusBadge.vue";
describe("status controls", () => {
  it("renders translated status", () => {
    expect(
      mount(StatusBadge, { props: { status: "running" } }).text(),
    ).toContain("执行中");
  });
  it("exposes progress semantics", () => {
    const wrapper = mount(ProgressBar, { props: { value: 62 } });
    expect(wrapper.attributes("aria-valuenow")).toBe("62");
    expect(wrapper.find("i").attributes("style")).toContain("62%");
  });
});

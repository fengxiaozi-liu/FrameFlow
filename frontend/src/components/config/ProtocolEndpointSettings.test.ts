import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ProtocolEndpointSettings from "./ProtocolEndpointSettings.vue";
import type { OperationBinding } from "../../api/types";
const protocol = {
  id: "dashscope.media-video.v1",
  operation: "video" as const,
  name: "视频协议",
  default_path: "/submit/video",
  polling_path: "/tasks/{task_id}",
};
function editor(binding: OperationBinding) {
  const wrapper = mount(ProtocolEndpointSettings, {
    props: {
      baseUrl: "https://example.com:8443",
      modelValue: binding,
      protocol,
      "onUpdate:modelValue": (value: OperationBinding) =>
        wrapper.setProps({ modelValue: value }),
    },
  });
  return wrapper;
}
describe("builtin protocol paths", () => {
  it("shows server defaults without a misleading custom-path choice", () => {
    const w = editor({ protocol: protocol.id, source: "manual" });
    expect(w.text()).toContain("/submit/video");
    expect(w.text()).toContain("/tasks/{task_id}");
    expect(w.text()).toContain("https://example.com:8443/submit/video");
    expect(w.text()).toContain("https://example.com:8443/tasks/{task_id}");
    expect(w.find("input").exists()).toBe(false);
  });
  it("preserves legacy overrides until an explicit restore", async () => {
    const w = editor({
      protocol: protocol.id,
      source: "manual",
      endpoint: "/legacy/video",
    });
    expect(w.text()).toContain("历史路径配置");
    expect(w.text()).toContain("/legacy/video");
    await w.get("button").trigger("click");
    expect(w.props("modelValue").endpoint).toBeUndefined();
    expect(w.text()).toContain("/submit/video");
    const explicit = editor({
      protocol: protocol.id,
      source: "manual",
      endpoint_mode: "default",
      endpoint: "/stale",
    });
    expect(explicit.text()).not.toContain("/stale");
  });
});

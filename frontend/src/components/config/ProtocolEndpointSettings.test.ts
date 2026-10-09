import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import ProtocolEndpointSettings from './ProtocolEndpointSettings.vue';
import type { OperationBinding } from '../../api/types';

const protocol = { id: 'dashscope.media-video.v1', operation: 'video' as const, name: '视频协议', default_path: '/submit/video', polling_path: '/tasks/{task_id}' };
function editor(binding: OperationBinding) {
  const wrapper = mount(ProtocolEndpointSettings, {
    props: { modelValue: binding, protocol, 'onUpdate:modelValue': (value: OperationBinding) => wrapper.setProps({ modelValue: value }) },
  });
  return wrapper;
}
describe('protocol paths', () => {
  it('shows server-provided defaults and only exposes editing in custom mode', async () => {
    const w = editor({ protocol: protocol.id, source: 'manual' });
    expect(w.get('[aria-label="实际请求路径"]').text()).toContain('/submit/video');
    expect(w.get('[aria-label="实际请求路径"]').text()).toContain('/tasks/{task_id}');
    expect(w.find('input[aria-label="自定义请求路径"]').exists()).toBe(false);
    await w.findAll('input[type="radio"]')[1]!.setValue();
    const input = w.get('input[aria-label="自定义请求路径"]');
    expect(input.attributes()).toHaveProperty('required');
    expect(w.text()).toContain('请填写自定义路径');
    await input.setValue('/custom/video');
    expect(w.text()).toContain('/custom/video');
    expect(w.text()).toContain('/tasks/{task_id}');
    await w.findAll('input[type="radio"]')[0]!.setValue();
    expect(w.props('modelValue').endpoint).toBeUndefined();
    expect(w.text()).not.toContain('/custom/video');
    expect(w.text()).toContain('/submit/video');
  });
  it('reopens a legacy override as custom but honors explicit default with a stale endpoint', () => {
    const legacy = editor({ protocol: protocol.id, source: 'manual', endpoint: '/legacy/video' });
    expect((legacy.get('input[aria-label="自定义请求路径"]').element as HTMLInputElement).value).toBe('/legacy/video');
    const explicit = editor({ protocol: protocol.id, source: 'manual', endpoint_mode: 'default', endpoint: '/stale/video' });
    expect(explicit.find('input[aria-label="自定义请求路径"]').exists()).toBe(false);
    expect(explicit.text()).toContain('/submit/video');
    expect(explicit.text()).not.toContain('/stale/video');
  });
});

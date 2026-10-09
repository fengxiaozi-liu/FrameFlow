import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import ShotGenerationSettings from './ShotGenerationSettings.vue';
import type { Model } from '../../api/types';
const base: Model = { id: 'm', connection_id: 'c', remote_id: 'arbitrary', name: '自定义视频', capabilities: ['video'], source: 'manual', enabled: true, supported: true, default: false };
describe('model-driven video settings', () => {
 it('explains missing configuration instead of hiding the model', () => {
  const w = mount(ShotGenerationSettings, { props: { modelValue: { video_model_id: 'm', aspect_ratio: '16:9', duration_seconds: 6, resolution: '720P' }, models: [{ ...base, availability: { video: { available: false, code: 'configuration_missing', reason: '请配置协议', binding: { protocol: '', source: '' } } } }] }, global: { stubs: { RouterLink: true } } });
  expect(w.text()).toContain('请配置协议');
  expect(w.findAll('select')[1]!.attributes()).toHaveProperty('disabled');
  expect(w.emitted('ready')?.[0]).toEqual([false]);
 });
 it('changes incompatible values to declared choices when a model is selected', async () => {
  const w = mount(ShotGenerationSettings, { props: { modelValue: { video_model_id: '', aspect_ratio: '16:9', duration_seconds: 6, resolution: '720P' }, models: [{ ...base, availability: { video: { available: true, code: 'configured', reason: '', binding: { protocol: 'dashscope.media-video.v1', source: 'manual', input_mode: 'first_frame', durations: [4, 8], resolutions: ['1080P'], ratios: ['9:16'], max_references: 1 } } } }] }, global: { stubs: { RouterLink: true } } });
  await w.findAll('select')[0]!.setValue('m');
  expect(w.emitted('update:modelValue')?.[0]?.[0]).toMatchObject({ video_model_id: 'm', duration_seconds: 4, resolution: '1080P', aspect_ratio: '9:16' });
 });
});

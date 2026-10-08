import {mount,flushPromises} from '@vue/test-utils';
import {createPinia,setActivePinia} from 'pinia';
import {defineComponent} from 'vue';
import {describe,it,expect,vi,beforeEach} from 'vitest';
import {useGenerationRuns} from './useGenerationRuns';
import {bindingsApi} from '../api/bindings';
vi.mock('../api/bindings',()=>({bindingsApi:{run:vi.fn(),runs:vi.fn().mockResolvedValue({runs:[]}),getRun:vi.fn()}}));
describe('one submission one run',()=>{
 beforeEach(()=>{sessionStorage.clear();vi.clearAllMocks();setActivePinia(createPinia())});
 it('deduplicates concurrent clicks and reuses key after uncertain failure',async()=>{
  let state!:ReturnType<typeof useGenerationRuns>;const w=mount(defineComponent({setup(){state=useGenerationRuns({project:'p',draft:'d',scene:'s'});return()=>null}}));await flushPromises();
  const input={expected_version:0,model_id:'im',video_model_id:'vid',path:'reference_image_then_video' as const};
  vi.mocked(bindingsApi.run).mockRejectedValueOnce(new Error('network interrupted'));
  await Promise.all([state.submit(input),state.submit(input)]);expect(bindingsApi.run).toHaveBeenCalledTimes(1);expect(state.pendingSubmission.value).toBe(true);
  const key=vi.mocked(bindingsApi.run).mock.calls[0][3].idempotency_key;
  vi.mocked(bindingsApi.run).mockResolvedValue({id:'run',scene_id:'s',state:'preparing',task_ids:[],snapshot_id:'snap'});
  await state.submit(input);expect(vi.mocked(bindingsApi.run).mock.calls[1][3].idempotency_key).toBe(key);expect(state.runs.value).toHaveLength(1);expect(state.pendingSubmission.value).toBe(false);w.unmount();
 });
 it('late list response cannot erase a just submitted run',async()=>{
  let resolve!:(v:{runs:[]})=>void;vi.mocked(bindingsApi.runs).mockReturnValueOnce(new Promise(r=>resolve=r));
  let state!:ReturnType<typeof useGenerationRuns>;const w=mount(defineComponent({setup(){state=useGenerationRuns({project:'p',draft:'d',scene:'s'});return()=>null}}));
  vi.mocked(bindingsApi.run).mockResolvedValue({id:'new',scene_id:'s',state:'preparing',task_ids:[],snapshot_id:'snap'});await state.submit({expected_version:0,model_id:'im',video_model_id:'vid',path:'reference_image_then_video'});
  resolve({runs:[]});await flushPromises();expect(state.runs.value.map(r=>r.id)).toEqual(['new']);w.unmount();
 });

});

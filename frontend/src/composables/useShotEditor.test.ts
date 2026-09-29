import {mount,flushPromises} from '@vue/test-utils';
import {defineComponent} from 'vue';
import {describe,it,expect,vi,beforeEach} from 'vitest';
import {useShotEditor} from './useShotEditor';
import {bindingsApi} from '../api/bindings';
import type {Draft,Scene} from '../api/types';
vi.mock('../api/bindings',()=>({bindingsApi:{saveContext:vi.fn()}}));
describe('shot save concurrency',()=>{
 beforeEach(()=>{localStorage.clear();vi.clearAllMocks()});
 it('keeps input typed while saving and saves the next version',async()=>{
  const scene:Scene={id:'s',order:1,title:'',visual_prompt:'old',narration:'',duration_seconds:4};const draft:Draft={id:'d',name:'d',version:1,story:{body:'',summary:'',scenes:[scene],updated_at:''}};
  let editor!:ReturnType<typeof useShotEditor>;const w=mount(defineComponent({setup(){editor=useShotEditor(()=>({project:'p',draft,scene}),()=>{});return()=>null}}));
  let resolve!:(v:Awaited<ReturnType<typeof bindingsApi.saveContext>>)=>void;vi.mocked(bindingsApi.saveContext).mockImplementationOnce(()=>new Promise(r=>resolve=r) as ReturnType<typeof bindingsApi.saveContext>);
  editor.change({prompt_document:{schema_version:1,nodes:[{type:'text',text:'first'}]}});const pending=editor.flush();
  editor.change({prompt_document:{schema_version:1,nodes:[{type:'text',text:'second'}]}});resolve({...draft,version:2} as Awaited<ReturnType<typeof bindingsApi.saveContext>>);await pending;await flushPromises();
  expect(editor.state.value.prompt_document.nodes[0]).toEqual({type:'text',text:'second'});expect(editor.dirty.value).toBe(true);
  vi.mocked(bindingsApi.saveContext).mockResolvedValue({...draft,version:3} as never);await editor.flush();expect(bindingsApi.saveContext).toHaveBeenLastCalledWith('p','d','s',expect.objectContaining({expected_version:2}));expect(editor.dirty.value).toBe(false);w.unmount();
 });
});

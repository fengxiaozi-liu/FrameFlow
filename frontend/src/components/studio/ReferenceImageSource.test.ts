import {mount,flushPromises} from '@vue/test-utils';
import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import ReferenceImageSource from './ReferenceImageSource.vue';
import {bindingsApi} from '../../api/bindings';
import {api} from '../../api/client';
import type {Material,Model,Task} from '../../api/types';
vi.mock('../../api/bindings',()=>({bindingsApi:{referenceImage:vi.fn(),task:vi.fn()}}));
vi.mock('../../api/client',()=>({api:{materials:vi.fn(),material:vi.fn(),mediaUrl:(s:string)=>s,retryTask:vi.fn(),cancelTask:vi.fn()}}));
const entity={id:'person',kind:'character' as const,name:'林夏',description:'短发，蓝外套',revision:1,references:[]};
const models=[{id:'im',enabled:true,supported:true,capabilities:['image'],name:'生图模型'}] as Model[];
const props={projectId:'p',draftId:'d',entity,models,materials:[] as Material[]};
describe('reference image sources',()=>{
 beforeEach(()=>{vi.clearAllMocks();localStorage.clear();vi.useFakeTimers()});afterEach(()=>vi.useRealTimers());
 it('filters the library and leaves reference adoption explicit',async()=>{
  const person={id:'m',name:'人物图',kind:'character',url:'/media/m.png'} as Material;
  vi.mocked(api.materials).mockResolvedValue({materials:[person,{...person,id:'scene',kind:'scene'}]});
  const w=mount(ReferenceImageSource,{props});await w.get('button').trigger('click');await flushPromises();
  expect(w.findAll('.reference-library-grid button')).toHaveLength(1);expect(w.emitted('add')).toBeUndefined();await w.get('.reference-library-grid button').trigger('click');expect(w.emitted('add')![0]).toEqual([person]);w.unmount();
 });
 it('replays lost submission with frozen input and ignores late results after unmount',async()=>{
  const task={id:'task',status:'running',progress:10} as Task;let resolve!: (value:Task)=>void;
  vi.mocked(bindingsApi.referenceImage).mockRejectedValueOnce(Error('连接中断')).mockResolvedValueOnce(task);
  vi.mocked(bindingsApi.task).mockImplementation(()=>new Promise(r=>{resolve=r}));
  const w=mount(ReferenceImageSource,{props});await w.findAll('button')[1]!.trigger('click');
  await w.get('button.primary').trigger('click');await flushPromises();expect(w.text()).toContain('恢复提交');
  await w.setProps({entity:{...entity,description:'修改后的描述'}});await w.get('button.primary').trigger('click');await flushPromises();
  const calls=vi.mocked(bindingsApi.referenceImage).mock.calls;expect(calls).toHaveLength(2);expect(calls[0]![2]).toEqual(calls[1]![2]);expect(calls[1]![2].prompt).toBe('短发，蓝外套');
  w.unmount();resolve({...task,status:'succeeded'});await flushPromises();await vi.advanceTimersByTimeAsync(4000);expect(api.material).not.toHaveBeenCalled();expect(w.emitted('add')).toBeUndefined();
 });
});

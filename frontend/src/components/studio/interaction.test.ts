import {mount} from '@vue/test-utils';
import {describe,it,expect} from 'vitest';
import MentionPromptEditor from './MentionPromptEditor.vue';
import ReferenceImageList from './ReferenceImageList.vue';
import DialogueVoicePanel from './DialogueVoicePanel.vue';
import EntityReferenceCard from './EntityReferenceCard.vue';
import GenerationRunList from './GenerationRunList.vue';
describe('binding editor interactions',()=>{
 it('keeps references and the default voice visible without a shot supplement field',()=>{
  const w=mount(EntityReferenceCard,{props:{entity:{id:'c',kind:'character',name:'林夏',description:'基础设定',revision:1,references:[]},binding:{id:'b',entity_id:'c',entity_revision:1,role:'cast'},materials:[],voices:[],inline:true}});
  expect(w.findAll('button').some(b=>b.text()==='折叠')).toBe(false);
  expect(w.find('textarea[placeholder]').exists()).toBe(false);expect(w.text()).not.toContain('本镜头补充');expect(w.text()).toContain('默认音色');
 });
 it('renders a single run as one task regardless of child count',()=>{
  const w=mount(GenerationRunList,{props:{runs:[{id:'r',scene_id:'s',state:'generating',task_ids:['im','vid','speech'],snapshot_id:'snap'}],base:'/studio/p/d',sceneId:'s'},global:{stubs:{RouterLink:{template:'<a><slot /></a>'}}}});
  expect(w.findAll('.generation-run-row')).toHaveLength(1);expect(w.text()).toContain('任务 001');expect(w.text()).not.toContain('任务 002');
 });
 it('renders text and immutable references inside a single editable area',async()=>{
  const w=mount(MentionPromptEditor,{props:{showInsertButton:false,modelValue:{schema_version:1,nodes:[{type:'text',text:'走进'},{type:'entity_ref',binding_id:'b1'},{type:'text',text:'门口'},{type:'text',text:''}]},options:[{id:'b1',name:'1111',kind:'location'}]}});
  expect(w.findAll('[role=textbox]')).toHaveLength(1);expect(w.findAll('textarea')).toHaveLength(0);expect(w.get('[role=textbox]').text()).toBe('走进@1111×门口');expect(w.get('.mention-chip').attributes('contenteditable')).toBe('false');
  await w.get('[role=textbox]').trigger('keydown',{key:'@'});expect(w.get('[role=option]').text()).toContain('1111');await w.get('[role=listbox]').trigger('keydown',{key:'Escape'});expect(w.find('[role=listbox]').exists()).toBe(false);w.unmount();
 });
 it('keeps literal markup as text and preserves bindings during Chinese composition',async()=>{
  const w=mount(MentionPromptEditor,{props:{modelValue:{schema_version:1,nodes:[{type:'text',text:'<img src=x>'},{type:'entity_ref',binding_id:'b'}]},options:[{id:'b',name:'<script>name</script>',kind:'character'}]}});
  expect(w.find('img').exists()).toBe(false);expect(w.find('script').exists()).toBe(false);
  const input=w.get('[role=textbox]');await input.trigger('compositionstart');input.element.append(document.createTextNode('中文'));await input.trigger('input');await input.trigger('keydown',{key:'@',isComposing:true});expect(w.find('[role=listbox]').exists()).toBe(false);expect(w.emitted('update:modelValue')).toBeUndefined();await input.trigger('compositionend');
  expect(w.emitted('update:modelValue')![0]![0]).toEqual({schema_version:1,nodes:[{type:'text',text:'<img src=x>'},{type:'entity_ref',binding_id:'b'},{type:'text',text:'中文'}]});w.unmount();
 });
 it('updates external documents and reference names without extra inputs',async()=>{
  const w=mount(MentionPromptEditor,{props:{modelValue:{schema_version:1,nodes:[{type:'text',text:'本地'}]},options:[]}});
  await w.setProps({modelValue:{schema_version:1,nodes:[{type:'entity_ref',binding_id:'b'},{type:'text',text:'服务器内容'},{type:'text'} as never]}});expect(w.get('.mention-chip').text()).toContain('引用失效');
  await w.setProps({options:[{id:'b',name:'林夏',kind:'character'}]});expect(w.get('.mention-chip').text()).toContain('林夏');expect(w.findAll('[role=textbox]')).toHaveLength(1);w.unmount();
 });
 it('reorders and removes only a binding',async()=>{
  const w=mount(ReferenceImageList,{props:{ids:['a','b'],materials:[]}});await w.findAll('[aria-label="参考图后移"]')[0]!.trigger('click');expect(w.emitted('update:ids')![0]).toEqual([['b','a']]);
  await w.findAll('[aria-label="移除参考图绑定"]')[0]!.trigger('click');expect(w.emitted('update:ids')![1]).toEqual([['b']]);
 });
 it('resets explicit voice override to inheritance',async()=>{
  const w=mount(DialogueVoicePanel,{props:{lines:[{id:'u',speaker_type:'character',speaker_id:'c',speaker_revision:1,text:'hello',order:0,voice_profile_id:'v',voice_profile_revision:1}],characters:[],voices:[]}});
  await w.get('[aria-label="选择音色"]').setValue('');const result=w.emitted('change')![0]![0] as {voice_profile_id?:string}[];expect(result[0]!.voice_profile_id).toBeUndefined();
 });
});

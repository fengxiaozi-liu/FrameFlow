import {mount} from '@vue/test-utils';
import {describe,it,expect} from 'vitest';
import MentionPromptEditor from './MentionPromptEditor.vue';
import ReferenceImageList from './ReferenceImageList.vue';
import DialogueVoicePanel from './DialogueVoicePanel.vue';
describe('binding editor interactions',()=>{
 it('inserts a stable identity between text at the caret',async()=>{
  const w=mount(MentionPromptEditor,{props:{modelValue:{schema_version:1,nodes:[{type:'text',text:'hello world'}]},options:[{id:'b1',name:'同名',kind:'character'},{id:'b2',name:'同名',kind:'character'}]}});
  const input=w.get('textarea');(input.element as HTMLTextAreaElement).setSelectionRange(6,6);await input.trigger('keydown',{key:'@'});
  await w.findAll('[role=option]')[1]!.trigger('click');
  const result=w.emitted('update:modelValue')![0]![0] as {nodes:unknown[]};
  expect(result.nodes).toEqual([{type:'text',text:'hello '},{type:'entity_ref',binding_id:'b2'},{type:'text',text:'world'}]);
 });
 it('does not open mentions while composing Chinese',async()=>{
  const w=mount(MentionPromptEditor,{props:{modelValue:{schema_version:1,nodes:[{type:'text',text:''}]},options:[]}});
  await w.get('textarea').trigger('keydown',{key:'@',isComposing:true});expect(w.find('[role=listbox]').exists()).toBe(false);
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

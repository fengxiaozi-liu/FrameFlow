import {mount} from '@vue/test-utils';
import {describe,it,expect} from 'vitest';
import CharacterVoiceSettings from './CharacterVoiceSettings.vue';
import type {Material} from '../../api/types';

const entity={id:'c',kind:'character' as const,name:'林夏',description:'短发',revision:1,references:[]};
const materials=[{id:'voice',kind:'voice',name:'温柔女声',url:'/media/voice.wav'},{id:'music',kind:'music',name:'音乐',url:'/media/music.wav'},{id:'image',kind:'character',name:'角色图',url:'/media/image.png'}] as Material[];
describe('character voice material selection',()=>{
 it('selects only voice materials, previews and clears without model configuration',async()=>{
  const w=mount(CharacterVoiceSettings,{props:{entity,materials}});
  expect(w.findAll('select')).toHaveLength(1);
  expect(w.findAll('option').map(o=>o.text())).toEqual(['暂不设置','温柔女声']);
  await w.get('select').setValue('voice');expect(w.vm.selectedMaterialID()).toBe('voice');expect(w.get('audio').attributes('src')).toContain('/media/voice.wav');
  await w.get('button').trigger('click');expect(w.vm.selectedMaterialID()).toBeUndefined();expect(w.find('audio').exists()).toBe(false);
  expect(w.text()).not.toContain('语音模型');w.unmount();
 });
 it('retains a saved selection and rejects missing materials until cleared',async()=>{
  const w=mount(CharacterVoiceSettings,{props:{entity:{...entity,voice_material_id:'voice'},materials}});
  expect(w.vm.selectedMaterialID()).toBe('voice');await w.setProps({materials:[]});expect(w.text()).toContain('素材库中暂无配音素材');expect(()=>w.vm.selectedMaterialID()).toThrow('不可用');
  await w.get('select').setValue('');expect(w.vm.selectedMaterialID()).toBeUndefined();w.unmount();
 });
});

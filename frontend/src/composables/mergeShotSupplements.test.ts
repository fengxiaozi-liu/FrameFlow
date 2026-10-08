import {describe,it,expect} from 'vitest';
import {mergeShotSupplements} from './mergeShotSupplements';
import type {ShotContext} from '../api/bindings';

describe('legacy supplement migration',()=>{
 it('keeps text and references, clears hidden fields, and never appends twice',()=>{
  const context:ShotContext={expected_version:1,title:'雨夜',prompt_document:{schema_version:1,nodes:[{type:'text',text:'原描述'}]},utterances:[],entity_bindings:[{id:'b',entity_id:'c',entity_revision:1,role:'cast',description_supplement:'湿发'}]};
  expect(mergeShotSupplements(context)).toBe(true);
  expect(context.prompt_document.nodes).toEqual([{type:'text',text:'原描述\n'},{type:'entity_ref',binding_id:'b'},{type:'text',text:'：湿发'}]);
  expect(context.entity_bindings[0]?.description_supplement).toBeUndefined();
  expect(mergeShotSupplements(context)).toBe(false);
  expect(context.prompt_document.nodes).toHaveLength(3);
 });
});

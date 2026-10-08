import type {ShotContext} from '../api/bindings';

// Called after loading either a server scene or the local unsaved draft.
// Clearing the source field makes this safe to repeat after refresh/conflict.
export function mergeShotSupplements(context:ShotContext):boolean {
 let changed=false;
 for(const binding of context.entity_bindings){
  if(!binding.description_supplement)continue;
  const nodes=context.prompt_document.nodes,last=nodes.at(-1);
  if(last?.type==='text')last.text+='\n';
  else if(nodes.length)nodes.push({type:'text',text:'\n'});
  nodes.push(
   {type:'entity_ref',binding_id:binding.id},
   {type:'text',text:`：${binding.description_supplement}`},
  );
  delete binding.description_supplement;
  changed=true;
 }
 return changed;
}

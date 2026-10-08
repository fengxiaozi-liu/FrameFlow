<script setup lang="ts">
import {nextTick,onMounted,ref,watch} from 'vue';
import type {PromptDocument} from '../../api/bindings';

const props=withDefaults(defineProps<{modelValue:PromptDocument;options:{id:string;name:string;kind:string}[];disabled?:boolean;allowInsert?:boolean;showInsertButton?:boolean}>(),{allowInsert:true,showInsertButton:true});
const emit=defineEmits<{'update:modelValue':[PromptDocument]}>();
const root=ref<HTMLElement>(),editor=ref<HTMLElement>();
const picker=ref(false),active=ref(0);
let savedRange:Range|undefined,composing=false;
const clean=(text:string|undefined)=>(text||'').replace(/\u200b/g,'');

// Vue owns the shell; the browser owns the editable DOM and its undo history.
// Do not redraw on our own model updates: that would reset the typing caret.
function read(container:Node=editor.value!):PromptDocument{
 const nodes:PromptDocument['nodes']=[];
 function text(value:string){value=clean(value);if(!value)return;const last=nodes[nodes.length-1];if(last?.type==='text')last.text+=value;else nodes.push({type:'text',text:value})}
 function walk(parent:Node){
  for(const child of Array.from(parent.childNodes)){
   if(child.nodeType===Node.TEXT_NODE){text(child.textContent||'');continue}
   if(!(child instanceof HTMLElement))continue;
   if(child.dataset.bindingId){nodes.push({type:'entity_ref',binding_id:child.dataset.bindingId});continue}
   if(child.tagName==='BR'){text('\n');continue}
   if((child.tagName==='DIV'||child.tagName==='P')&&nodes.length)text('\n');
   walk(child);
  }
 }
 walk(container);return {schema_version:1,nodes:nodes.length?nodes:[{type:'text',text:''}]};
}
function normalized(value:PromptDocument){const nodes:PromptDocument['nodes']=[];for(const node of value.nodes){if(node.type==='text'){const value=clean(node.text);if(!value)continue;const last=nodes[nodes.length-1];if(last?.type==='text')last.text+=value;else nodes.push({type:'text',text:value})}else nodes.push({...node})}return JSON.stringify(nodes)}
function chip(id:string){
 const element=document.createElement('span');element.className='mention-chip';element.setAttribute('contenteditable','false');element.dataset.bindingId=id;
 const label=document.createElement('span');label.className='mention-label';label.textContent='@'+(props.options.find(o=>o.id===id)?.name||'引用失效');element.append(label);
 const remove=document.createElement('button');remove.type='button';remove.className='mention-remove';remove.tabIndex=-1;remove.setAttribute('aria-label','移除描述引用');remove.textContent='×';remove.disabled=!!props.disabled;element.append(remove);
 return element;
}
function render(){
 if(!editor.value||composing)return;
 if(normalized(read())===normalized(props.modelValue))return;
 const fragment=document.createDocumentFragment();
 for(const node of props.modelValue.nodes){fragment.append(node.type==='text'?document.createTextNode(node.text||''):chip(node.binding_id))}
 if(fragment.lastChild instanceof HTMLElement)fragment.append(document.createTextNode('\u200b'));
 editor.value.replaceChildren(fragment);
}
function sync(){if(!composing&&!props.disabled)emit('update:modelValue',read())}
function selection(){const s=window.getSelection();if(s?.rangeCount&&editor.value?.contains(s.getRangeAt(0).commonAncestorContainer))return s.getRangeAt(0).cloneRange();return undefined}
function restore(){editor.value?.focus();const s=window.getSelection();if(!s||!editor.value)return;const range=savedRange&&editor.value.contains(savedRange.commonAncestorContainer)?savedRange:document.createRange();if(range!==savedRange){range.selectNodeContents(editor.value);range.collapse(false)}s.removeAllRanges();s.addRange(range)}
function open(){if(props.disabled||!props.allowInsert)return;savedRange=selection();picker.value=true;active.value=0;void nextTick(()=>root.value?.querySelector<HTMLElement>('[role=option],.mention-picker button')?.focus())}
function close(){picker.value=false;restore()}
function insert(id:string){
 if(props.disabled)return;
 restore();
 // insertHTML uses escaped DOM-created markup and preserves browser Undo/Redo.
 document.execCommand('insertHTML',false,chip(id).outerHTML+'&#8203;');
 picker.value=false;savedRange=selection();sync();
}
function deleteAdjacent(backward:boolean){
 const range=selection();if(!range?.collapsed||!editor.value)return false;
 let node:Node|null=range.startContainer,adjacent:Node|null;
 if(node.nodeType===Node.TEXT_NODE){
  const value=node.textContent||'';
  if(clean(backward?value.slice(0,range.startOffset):value.slice(range.startOffset)))return false;
  adjacent=backward?node.previousSibling:node.nextSibling;
 }else adjacent=node.childNodes[range.startOffset-(backward?1:0)]||null;
 while(!adjacent&&node!==editor.value){node=node.parentNode;if(!node||node===editor.value)break;adjacent=backward?node.previousSibling:node.nextSibling}
 while(adjacent?.nodeType===Node.TEXT_NODE&&!clean(adjacent.textContent||''))adjacent=backward?adjacent.previousSibling:adjacent.nextSibling;
 if(!(adjacent instanceof HTMLElement)||!adjacent.dataset.bindingId)return false;
 if(backward)range.setStartBefore(adjacent);else range.setEndAfter(adjacent);
 savedRange=range;restore();document.execCommand('delete');sync();return true;
}
function key(event:KeyboardEvent){
 if(props.disabled||composing||event.isComposing)return;
 if((event.key==='Backspace'||event.key==='Delete')&&!event.ctrlKey&&!event.metaKey&&deleteAdjacent(event.key==='Backspace')){event.preventDefault()}
 else if(props.allowInsert&&event.key==='@'){event.preventDefault();open()}
 else if(event.key==='Enter'){event.preventDefault();document.execCommand('insertText',false,'\n');sync()}
 else if(event.key==='Escape'&&picker.value){event.preventDefault();close()}
}
function pickerKey(event:KeyboardEvent){if(event.isComposing)return;if(event.key==='Escape'){event.preventDefault();close()}else if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();active.value=Math.max(0,Math.min(props.options.length-1,active.value+(event.key==='ArrowDown'?1:-1)));root.value?.querySelectorAll<HTMLElement>('[role=option]')[active.value]?.focus()}}
function click(event:MouseEvent){const target=event.target as HTMLElement;if(!target.closest('.mention-remove')||props.disabled)return;const token=target.closest<HTMLElement>('[data-binding-id]');if(!token)return;const range=document.createRange();range.selectNode(token);savedRange=range;restore();document.execCommand('delete');sync()}
function paste(event:ClipboardEvent){event.preventDefault();if(props.disabled)return;document.execCommand('insertText',false,event.clipboardData?.getData('text/plain')||'');sync()}
function copy(event:ClipboardEvent){const range=selection();if(!range||range.collapsed)return;event.preventDefault();const value=read(range.cloneContents()).nodes.map(n=>n.type==='text'?n.text:'@'+(props.options.find(o=>o.id===n.binding_id)?.name||'引用失效')).join('');event.clipboardData?.setData('text/plain',value)}
function compositionEnd(){composing=false;sync()}
onMounted(render);
watch(()=>props.modelValue,render,{deep:true});
watch(()=>[props.options,props.disabled],()=>{for(const token of editor.value?.querySelectorAll<HTMLElement>('[data-binding-id]')||[]){const label=token.querySelector('.mention-label');if(label)label.textContent='@'+(props.options.find(o=>o.id===token.dataset.bindingId)?.name||'引用失效');const button=token.querySelector('button');if(button)button.disabled=!!props.disabled}},{deep:true});
</script>

<template><div ref="root" class="mention-editor">
 <div ref="editor" class="prompt-input" role="textbox" aria-label="镜头描述文本" aria-multiline="true" :aria-disabled="disabled||undefined" :contenteditable="!disabled" :tabindex="disabled?-1:0" data-placeholder="描述画面内容，输入 @ 引用场景或人物" @input="sync" @keydown="key" @click="click" @paste="paste" @copy="copy" @drop.prevent @compositionstart="composing=true" @compositionend="compositionEnd" />
 <button v-if="allowInsert&&showInsertButton" type="button" :disabled="disabled" @mousedown.prevent="savedRange=selection()" @click="open">＠ 插入场景或人物</button>
 <div v-if="picker" class="mention-picker" role="listbox" aria-label="引用候选" @keydown="pickerKey"><p v-if="!options.length">请先添加场景或人物分组。</p><button v-for="(option,i) in options" :key="option.id" type="button" role="option" :aria-selected="i===active" @focus="active=i" @click="insert(option.id)">{{option.kind==='location'?'场景':'人物'}} · {{option.name}} · {{option.id.slice(-6)}}</button><button type="button" @click="close">取消</button></div>
</div></template>

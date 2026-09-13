import { defineStore } from "pinia";
import { ref } from "vue";
import type { Draft, Scene } from "../api/types";
export const useWorkspaceStore = defineStore("workspace", () => {
  const idea = ref(""),
    projectId = ref(""),
    storyProvider = ref(""),
    imageProvider = ref(""),
    videoProvider = ref(""),
    activeResource = ref("character"),
    aspectRatio = ref("9:16"),
    subtitles = ref(true),
    bgmVolume = ref(25),
    frameSource = ref("generated"),
    selectedMaterials = ref<Record<string, string>>({}),
    draft = ref<Draft>({
      id: "draft-local",
      name: "未命名视频",
      story: {
        summary: "",
        body: "",
        scenes: [],
        updated_at: new Date().toISOString(),
      },
    });
  function addScene() {
    const order = draft.value.story.scenes.length + 1;
    draft.value.story.scenes.push({
      id: `scene-${Date.now()}`,
      order,
      title: `分镜 ${order}`,
      visual_prompt: "",
      narration: "",
      duration_seconds: 10,
    } as Scene);
  }
  return {
    idea,
    projectId,
    storyProvider,
    imageProvider,
    videoProvider,
    activeResource,
    aspectRatio,
    subtitles,
    bgmVolume,
    frameSource,
    selectedMaterials,
    draft,
    addScene,
  };
});

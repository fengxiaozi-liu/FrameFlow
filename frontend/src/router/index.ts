import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import StudioView from "../views/StudioView.vue";
import TasksView from "../views/TasksView.vue";
import ConfigView from "../views/ConfigView.vue";
import MaterialsView from "../views/MaterialsView.vue";
import ShotListView from '../views/ShotListView.vue';
import ShotDetailView from '../views/ShotDetailView.vue';
import GenerationRunView from '../views/GenerationRunView.vue';
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: HomeView },
    { path: "/studio", component: StudioView },
    { path: "/studio/:projectId/:draftId", component: StudioView },
    { path: '/studio/:projectId/:draftId/shots', component: ShotListView },
    { path: '/studio/:projectId/:draftId/shots/:sceneId', component: ShotDetailView },
    { path: '/studio/:projectId/:draftId/shots/:sceneId/runs/:runId', component: GenerationRunView },
    { path: "/tasks", component: TasksView },
    { path: "/materials", component: MaterialsView },
    { path: "/config", component: ConfigView },
  ],
});

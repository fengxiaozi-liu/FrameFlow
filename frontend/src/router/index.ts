import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import StudioView from "../views/StudioView.vue";
import TasksView from "../views/TasksView.vue";
import ConfigView from "../views/ConfigView.vue";
import MaterialsView from "../views/MaterialsView.vue";
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: HomeView },
    { path: "/studio", component: StudioView },
    { path: "/studio/:projectId/:draftId", component: StudioView },
    { path: "/tasks", component: TasksView },
    { path: "/materials", component: MaterialsView },
    { path: "/config", component: ConfigView },
  ],
});

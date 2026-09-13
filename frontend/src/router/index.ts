import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import StudioView from "../views/StudioView.vue";
import TasksView from "../views/TasksView.vue";
import ConfigView from "../views/ConfigView.vue";
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: HomeView },
    { path: "/studio", component: StudioView },
    { path: "/tasks", component: TasksView },
    { path: "/config", component: ConfigView },
  ],
});

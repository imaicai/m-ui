/**
 * 注册全局组件
 */

import Vue from 'vue';
import { Icon } from './components/Icon.vue';

// 注册全局图标组件
Vue.component('Icon', Icon);

// 可以在这里添加其他全局组件
// Vue.component('ComponentName', Component);
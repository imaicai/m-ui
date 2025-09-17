<template>
  <span class="icon" :style="iconStyle" v-html="iconSvg"></span>
</template>

<script>
import { ICONS } from '../icons';

export default {
  name: 'Icon',
  props: {
    name: { type: String, required: true },
    size: { type: [Number, String], default: 24 },
    weight: { type: String, default: 'regular' }, // regular, fill
    color: { type: String, default: 'currentColor' },
  },
  computed: {
    iconSvg() {
      const icon = ICONS[this.name];
      if (!icon) return '';
      
      return icon
        .replace('width="24"', `width="${this.size}"`)
        .replace('height="24"', `height="${this.size}"`)
        .replace('fill="currentColor"', `fill="${this.color}"`);
    },
    iconStyle() {
      return {
        '--icon-size': typeof this.size === 'number' ? `${this.size}px` : this.size,
        '--icon-color': this.color,
      };
    }
  }
};
</script>

<style scoped>
.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--icon-size, 24px);
  height: var(--icon-size, 24px);
  color: var(--icon-color, currentColor);
  transition: color var(--transition-fast) var(--easing-standard);
}
</style>
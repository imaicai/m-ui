/**
 * 扁平插画设计风格 - 图标系统
 * 基于Phosphor Icons的SVG图标库
 */

export const ICONS = {
  // 系统图标
  dashboard: `<svg width="24" height="24" viewBox="0 0 256 256"><path fill="currentColor" d="M208 32H48a16 16 0 0 0-16 16v160a16 16 0 0 0 16 16h160a16 16 0 0 0 16-16V48a16 16 0 0 0-16-16Zm0 16v48H48V48h160ZM48 160h160v48H48v-48Z"/></svg>`,
  settings: `<svg width="24" height="24" viewBox="0 0 256 256"><path fill="currentColor" d="M..."/></svg>`,
  // 更多图标...
};

// 图标组件
export const Icon = {
  props: {
    name: { type: String, required: true },
    size: { type: Number, default: 24 },
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
    }
  },
  template: `<span class="icon" v-html="iconSvg"></span>`
};
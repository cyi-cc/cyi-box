<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { NButton, NColorPicker, NIcon, NSelect, useMessage } from 'naive-ui'
import { CopyOutline, DiceOutline, ImageOutline } from '@vicons/ionicons5'
import { copyText } from '../../lib/kit'

const message = useMessage()

const dirOptions = [
  { label: '↗ 对角', value: '135deg' },
  { label: '→ 横向', value: '90deg' },
  { label: '↓ 纵向', value: '180deg' },
  { label: '↘ 反对角', value: '45deg' },
  { label: '○ 径向', value: 'radial' }
]
const dir = ref('135deg')

const presets: { name: string; colors: string[] }[] = [
  { name: '暖阳橙', colors: ['#ff9966', '#ff5e62'] },
  { name: '蜜桃苏打', colors: ['#ffecd2', '#fcb69f'] },
  { name: '蓝莓之夜', colors: ['#30cfd0', '#330867'] },
  { name: '薄荷花', colors: ['#a8edea', '#fed6e3'] },
  { name: '落日熔金', colors: ['#f6d365', '#fda085'] },
  { name: '紫罗兰梦', colors: ['#a18cd1', '#fbc2eb'] },
  { name: '青柠气泡', colors: ['#96fbc4', '#f9f586'] },
  { name: '深空', colors: ['#243949', '#517fa4'] },
  { name: '草莓奶昔', colors: ['#ff9a9e', '#fecfef'] },
  { name: '极光', colors: ['#43e97b', '#38f9d7'] },
  { name: '暮山紫', colors: ['#667eea', '#764ba2'] },
  { name: '芒果冰', colors: ['#ffe259', '#ffa751'] },
  { name: '冰川', colors: ['#e0eafc', '#cfdef3'] },
  { name: '玫瑰盐', colors: ['#f77062', '#fe5196'] },
  { name: '青瓷', colors: ['#89f7fe', '#66a6ff'] },
  { name: '拿铁', colors: ['#c79081', '#dfa579'] },
  { name: '葡萄柚', colors: ['#ff758c', '#ff7eb3'] },
  { name: '抹茶', colors: ['#d4fc79', '#96e6a1'] },
  { name: '霓虹', colors: ['#fddb92', '#d1fdff'] },
  { name: '熔岩', colors: ['#f83600', '#f9d423'] },
  { name: '雾都', colors: ['#757f9a', '#d7dde8'] },
  { name: '汽水', colors: ['#84fab0', '#8fd3f4'] },
  { name: '葡萄汁', colors: ['#7028e4', '#e5b2ca'] },
  { name: '奶油', colors: ['#fff1eb', '#ace0f9'] }
]

function css(colors: string[], d = dir.value): string {
  if (d === 'radial') return `radial-gradient(circle, ${colors.join(', ')})`
  return `linear-gradient(${d}, ${colors.join(', ')})`
}

async function grab(colors: string[]) {
  const text = `background: ${css(colors)};`
  message.success(await copyText(text) ? '已复制 CSS' : '复制失败')
}

// 随机渐变：ref 数组，randCount 整批重摇；reroll 单张重摇
const randCount = ref(0)
const hex = () => '#' + Math.floor(Math.random() * 0xffffff).toString(16).padStart(6, '0')
const randomList = ref(Array.from({ length: 8 }, () => [hex(), hex()]))
watchEffect(() => {
  if (randCount.value > 0) randomList.value = Array.from({ length: 8 }, () => [hex(), hex()])
})
const randoms = computed(() => randomList.value)
function reroll(i: number) {
  randomList.value[i] = [hex(), hex()]
}

// 自定义
const customA = ref('#43e97b')
const customB = ref('#38f9d7')
const customC = ref('')
const customColors = computed(() => (customC.value ? [customA.value, customC.value, customB.value] : [customA.value, customB.value]))
</script>

<template>
  <div class="grad-page">
    <div class="panel grad-custom">
      <div class="grad-preview" :style="{ background: css(customColors) }"></div>
      <div class="grad-controls">
        <div class="panel-title"><n-icon style="vertical-align:-2px"><ImageOutline /></n-icon> 自定义渐变</div>
        <div style="display:flex;gap:14px;flex-wrap:wrap;align-items:center">
          <n-color-picker v-model:value="customA" style="width:170px" />
          <n-color-picker v-model:value="customC" style="width:170px" />
          <n-color-picker v-model:value="customB" style="width:170px" />
          <n-select v-model:value="dir" :options="dirOptions" style="width:120px" />
          <n-button type="primary" @click="grab(customColors)">
            <template #icon><n-icon><CopyOutline /></n-icon></template>复制 CSS
          </n-button>
        </div>
        <div class="grad-css mono">background: {{ css(customColors) }};</div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">
        渐变图鉴 <span class="hint">点卡片复制 CSS，方向跟着上面选</span>
      </div>
      <div class="grad-grid">
        <div v-for="g in presets" :key="g.name" class="grad-card" @click="grab(g.colors)">
          <div class="grad-swatch" :style="{ background: css(g.colors) }"></div>
          <div class="grad-info">
            <b>{{ g.name }}</b>
            <span class="mono">{{ g.colors.join(' → ') }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">
        随机灵感 <span class="hint">点卡片复制 CSS，骰子只换这一张</span>
        <span style="flex:1"></span>
        <n-button size="small" @click="randCount++">
          <template #icon><n-icon><DiceOutline /></n-icon></template>换一批随机
        </n-button>
      </div>
      <div class="grad-grid">
        <div v-for="(colors, i) in randoms" :key="i" class="grad-card" @click="grab(colors)">
          <div class="grad-swatch" :style="{ background: css(colors) }"></div>
          <div class="grad-info">
            <b>随机 #{{ i + 1 }}</b>
            <span class="mono">{{ colors.join(' → ') }}</span>
            <n-button size="tiny" quaternary class="reroll" @click.stop="reroll(i)">
              <template #icon><n-icon :size="13"><DiceOutline /></n-icon></template>
            </n-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.grad-page { display: flex; flex-direction: column; gap: 14px; }
.grad-custom { display: flex; gap: 18px; align-items: stretch; }
.grad-preview { width: 200px; min-height: 130px; border: var(--cb-border); border-radius: 14px; flex: none; }
.grad-controls { flex: 1; display: flex; flex-direction: column; gap: 12px; }
.grad-css {
  font-size: 12.5px; border: 2px dashed var(--cb-ink); border-radius: 8px;
  padding: 8px 12px; background: #fff; word-break: break-all;
}
.mono { font-family: Consolas, monospace; }
.grad-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 14px; }
.grad-card {
  position: relative;
  border: var(--cb-border); border-radius: 12px; overflow: hidden;
  cursor: pointer; background: #fff; box-shadow: var(--cb-shadow-sm);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.reroll { position: absolute; top: 6px; right: 6px; z-index: 1; }
.grad-card:hover { transform: translate(-3px, -3px); box-shadow: var(--cb-shadow); }
.grad-swatch { height: 90px; }
.grad-info { padding: 10px 12px; display: flex; flex-direction: column; gap: 3px; }
.grad-info b { font-size: 13.5px; }
.grad-info span { font-size: 11px; color: var(--cb-ink-3); }
@media (max-width: 800px) { .grad-custom { flex-direction: column; } .grad-preview { width: 100%; } }
</style>

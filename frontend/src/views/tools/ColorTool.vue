<script setup lang="ts">
import { computed, ref } from 'vue'
import { NIcon, useMessage } from 'naive-ui'
import { ColorPaletteOutline } from '@vicons/ionicons5'
import { copyText } from '../../lib/kit'

const message = useMessage()

// ---- HSL 为唯一状态源，色盘/图片/色板都写它 ----
const hue = ref(14)
const sat = ref(83)
const lit = ref(50)

function hslToHex(h: number, s: number, l: number): string {
  s /= 100; l /= 100
  const c = (1 - Math.abs(2 * l - 1)) * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = l - c / 2
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x]
    : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x]
  const to = (v: number) => Math.round((v + m) * 255).toString(16).padStart(2, '0')
  return `#${to(r)}${to(g)}${to(b)}`
}

const color = computed(() => hslToHex(hue.value, sat.value, lit.value))

function hexToRgb(hex: string): [number, number, number] | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return null
  const v = parseInt(m[1], 16)
  return [(v >> 16) & 255, (v >> 8) & 255, v & 255]
}

function rgbToHsl(r: number, g: number, b: number): [number, number, number] {
  r /= 255; g /= 255; b /= 255
  const max = Math.max(r, g, b), min = Math.min(r, g, b)
  const l = (max + min) / 2
  if (max === min) return [0, 0, Math.round(l * 100)]
  const d = max - min
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
  let h = 0
  if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) * 60
  else if (max === g) h = ((b - r) / d + 2) * 60
  else h = ((r - g) / d + 4) * 60
  return [Math.round(h), Math.round(s * 100), Math.round(l * 100)]
}

function setFromHex(hex: string) {
  const rgb = hexToRgb(hex)
  if (!rgb) return
  const [h, s, l] = rgbToHsl(...rgb)
  hue.value = h; sat.value = s; lit.value = l
}

// ---- 色盘：角度=色相，半径=饱和度；球跟手走 ----
const wheelEl = ref<HTMLElement | null>(null)

const dotPos = computed(() => {
  const rad = ((hue.value - 90) * Math.PI) / 180
  const r = (sat.value / 100) * 50
  return { x: 50 + Math.cos(rad) * r, y: 50 + Math.sin(rad) * r }
})

function onWheel(e: PointerEvent) {
  const el = wheelEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const cx = rect.width / 2, cy = rect.height / 2
  let dx = e.clientX - rect.left - cx
  let dy = e.clientY - rect.top - cy
  const dist = Math.min(Math.hypot(dx, dy), cx)
  hue.value = Math.round(((Math.atan2(dy, dx) * 180) / Math.PI + 90 + 360) % 360)
  sat.value = Math.round((dist / cx) * 100)
  if (lit.value <= 1 || lit.value >= 99) lit.value = 50 // 白/黑卡死时动色盘自动回中
}

// ---- 格式 / 阶梯 / 配色 / 色板 ----
const rgb = computed(() => hexToRgb(color.value))
const hsl = computed(() => [hue.value, sat.value, lit.value] as const)

const formats = computed(() => {
  if (!rgb.value) return []
  const [r, g, b] = rgb.value
  const [h, s, l] = hsl.value
  return [
    { label: 'HEX', value: color.value.toUpperCase() },
    { label: 'RGB', value: `rgb(${r}, ${g}, ${b})` },
    { label: 'RGBA', value: `rgba(${r}, ${g}, ${b}, 1)` },
    { label: 'HSL', value: `hsl(${h}, ${s}%, ${l}%)` }
  ]
})

function mix(hex: string, target: [number, number, number], t: number): string {
  const c = hexToRgb(hex) ?? [0, 0, 0]
  const to = (a: number, b: number) => Math.round(a + (b - a) * t).toString(16).padStart(2, '0')
  return `#${to(c[0], target[0])}${to(c[1], target[1])}${to(c[2], target[2])}`
}

const shades = computed(() => [
  ...[0.9, 0.7, 0.5, 0.3].map(t => mix(color.value, [255, 255, 255], t)),
  color.value,
  ...[0.25, 0.5, 0.75, 0.9].map(t => mix(color.value, [0, 0, 0], t))
])

const harmonies = computed(() => {
  const [h, s, l] = hsl.value
  return [
    { name: '互补色', list: [hslToHex((h + 180) % 360, s, l)] },
    { name: '类似色', list: [hslToHex((h + 330) % 360, s, l), hslToHex((h + 30) % 360, s, l)] },
    { name: '三分色', list: [hslToHex((h + 120) % 360, s, l), hslToHex((h + 240) % 360, s, l)] }
  ]
})

const presets = [
  '#ec5b13', '#f59e0b', '#fef08a', '#a7f3d0', '#10b981', '#bae6fd',
  '#3b82f6', '#6366f1', '#d9b3ff', '#a855f7', '#ffc1cc', '#f43f5e',
  '#1a1a1a', '#57534d', '#f8f6f6', '#ffffff'
]

async function pick(c: string) {
  setFromHex(c)
  message.success(await copyText(c.toUpperCase()) ? `${c.toUpperCase()} 已复制` : '复制失败')
}
</script>

<template>
  <div class="color-page">
    <div class="panel picker-panel">
      <div class="wheel-col">
        <div
          ref="wheelEl" class="wheel"
          @pointerdown="onWheel" @pointermove="onWheel"
        >
          <div class="wheel-hue"></div>
          <div class="wheel-sat"></div>
          <div class="wheel-dot" :style="{ left: dotPos.x + '%', top: dotPos.y + '%', background: color }"></div>
        </div>
      </div>
      <div class="picker-right">
        <div class="panel-title">
          <n-icon style="vertical-align:-2px"><ColorPaletteOutline /></n-icon> 色球取色
          <span class="hint">球跟手走 · 点任意色值复制</span>
        </div>
        <div class="big-swatch" :style="{ background: color }">
          <span class="swatch-hex">{{ color.toUpperCase() }}</span>
        </div>
        <div class="fmt-list">
          <div v-for="f in formats" :key="f.label" class="fmt-row" @click="pick(f.value)">
            <span class="fmt-k">{{ f.label }}</span><b>{{ f.value }}</b>
          </div>
        </div>
      </div>
    </div>

    <div class="grid2">
      <div class="panel">
        <div class="panel-title">明度阶梯 <span class="hint">浅 → 本色 → 深</span></div>
        <div class="shade-row">
          <div v-for="(s, i) in shades" :key="i" class="shade" :class="{ cur: s.toLowerCase() === color.toLowerCase() }"
            :style="{ background: s }" @click="pick(s)">
            <span>{{ s.slice(1).toUpperCase() }}</span>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-title">配色方案</div>
        <div v-for="h in harmonies" :key="h.name" class="harm-row">
          <span class="harm-name">{{ h.name }}</span>
          <div class="harm-colors">
            <div class="swatch-cell" :style="{ background: color }" @click="pick(color)"></div>
            <div v-for="c in h.list" :key="c" class="swatch-cell" :style="{ background: c }" @click="pick(c)"></div>
          </div>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-title">常用色板</div>
      <div class="preset-grid">
        <div v-for="c in presets" :key="c" class="preset" :style="{ background: c }" @click="pick(c)"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.color-page { display: flex; flex-direction: column; gap: 14px; }
.picker-panel { display: flex; gap: 26px; align-items: flex-start; }

/* ---- 色球 ---- */
.wheel-col { display: flex; flex-direction: column; gap: 12px; align-items: center; }
.wheel {
  position: relative; width: 240px; height: 240px; border-radius: 50%;
  border: var(--cb-border); box-shadow: var(--cb-shadow-sm);
  cursor: crosshair; touch-action: none; user-select: none; overflow: hidden;
}
.wheel-hue {
  position: absolute; inset: 0;
  background: conic-gradient(from -90deg,
    #f00 0deg, #ff0 60deg, #0f0 120deg, #0ff 180deg, #00f 240deg, #f0f 300deg, #f00 360deg);
}
.wheel-sat {
  position: absolute; inset: 0;
  background: radial-gradient(circle, #fff 0%, rgba(255,255,255,0) 72%);
}
.wheel-dot {
  position: absolute; width: 18px; height: 18px; border-radius: 50%;
  border: 3px solid #fff; box-shadow: 0 0 0 2px var(--cb-ink);
  transform: translate(-50%, -50%); pointer-events: none;
}

.picker-right { flex: 1; display: flex; flex-direction: column; gap: 14px; }
.big-swatch {
  width: 100%; max-width: 420px; height: 110px;
  border: var(--cb-border); border-radius: 16px;
  display: flex; align-items: flex-end; justify-content: center;
  transition: background 0.15s ease;
}
.swatch-hex {
  background: #fff; border: 2px solid var(--cb-ink); border-radius: 8px;
  font-weight: 900; font-size: 13px; padding: 3px 10px; margin-bottom: 10px;
}
.fmt-list { display: flex; flex-direction: column; gap: 8px; max-width: 420px; }
.fmt-row {
  display: flex; gap: 12px; align-items: center; cursor: pointer;
  border: 2px solid var(--cb-ink); border-radius: 8px; padding: 6px 12px; background: #fff;
  font-size: 13px;
}
.fmt-row:hover { background: var(--cb-yellow); }
.fmt-k { font-weight: 800; color: var(--cb-ink-3); min-width: 62px; font-size: 12px; }
.fmt-row b { font-family: Consolas, monospace; }

.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
@media (max-width: 900px) { .grid2 { grid-template-columns: 1fr; } }

.shade-row { display: grid; grid-template-columns: repeat(auto-fill, minmax(70px, 1fr)); gap: 6px; }
.shade {
  height: 56px; border: 2.5px solid var(--cb-ink); border-radius: 8px;
  cursor: pointer; display: flex; align-items: flex-end; justify-content: center;
  font-size: 10px; font-weight: 800; transition: transform 0.12s ease;
}
.shade span { background: rgba(255,255,255,0.85); border-radius: 4px; padding: 0 4px; margin-bottom: 4px; }
.shade:hover { transform: translateY(-3px); }
.shade.cur { outline: 3px solid var(--cb-primary); outline-offset: 2px; }
.harm-row { display: flex; align-items: center; gap: 14px; margin-bottom: 10px; }
.harm-name { font-weight: 800; font-size: 13px; min-width: 56px; }
.harm-colors { display: flex; gap: 8px; flex-wrap: wrap; }
.swatch-cell {
  width: 64px; height: 44px; border: 2.5px solid var(--cb-ink);
  border-radius: 8px; cursor: pointer; transition: transform 0.12s ease;
}
.swatch-cell:hover { transform: translateY(-3px); }
.preset-grid { display: grid; grid-template-columns: repeat(8, 1fr); gap: 8px; }
.preset {
  height: 52px; border: 2.5px solid var(--cb-ink); border-radius: 10px;
  cursor: pointer; transition: transform 0.12s ease;
}
.preset:hover { transform: translateY(-3px) scale(1.03); }
@media (max-width: 800px) {
  .picker-panel { flex-direction: column; }
  .shade-row { grid-template-columns: repeat(5, 1fr); }
  .preset-grid { grid-template-columns: repeat(4, 1fr); }
}
</style>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NIcon, useMessage } from 'naive-ui'
import { CloudUploadOutline, ImageOutline, CopyOutline } from '@vicons/ionicons5'
import { copyText } from '../../lib/kit'

const message = useMessage()

const fileInput = ref<HTMLInputElement | null>(null)
const canvasEl = ref<HTMLCanvasElement | null>(null)
const loupeEl = ref<HTMLCanvasElement | null>(null)
const hasImage = ref(false)
const hoverColor = ref('')
const hoverPos = ref({ x: 0, y: 0, lx: 0, ly: 0, show: false })
const picked = ref('')
const palette = ref<string[]>([])
let pressing = false

const LOUPE = 132      // 放大镜直径 px
const LOUPE_N = 11     // 放大区域边长（源像素数）

function openFile() { fileInput.value?.click() }

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const url = URL.createObjectURL(f)
  const img = new Image()
  img.onload = () => {
    URL.revokeObjectURL(url)
    const canvas = canvasEl.value
    if (!canvas) return
    const maxW = 640
    const scale = Math.min(1, maxW / img.width)
    canvas.width = Math.round(img.width * scale)
    canvas.height = Math.round(img.height * scale)
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx) return
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
    hasImage.value = true
    extractPalette(ctx, canvas.width, canvas.height)
  }
  img.src = url
  ;(e.target as HTMLInputElement).value = ''
}

function onDrop(e: DragEvent) {
  const f = e.dataTransfer?.files?.[0]
  if (!f || !f.type.startsWith('image/')) return
  const dt = new DataTransfer()
  dt.items.add(f)
  if (fileInput.value) {
    fileInput.value.files = dt.files
    onFile({ target: fileInput.value } as unknown as Event)
  }
}

function sampleAt(e: PointerEvent): string | null {
  const canvas = canvasEl.value
  if (!canvas) return null
  const rect = canvas.getBoundingClientRect()
  const x = Math.floor(((e.clientX - rect.left) / rect.width) * canvas.width)
  const y = Math.floor(((e.clientY - rect.top) / rect.height) * canvas.height)
  if (x < 0 || y < 0 || x >= canvas.width || y >= canvas.height) return null
  const ctx = canvas.getContext('2d', { willReadFrequently: true })
  if (!ctx) return null
  const d = ctx.getImageData(x, y, 1, 1).data
  const to = (v: number) => v.toString(16).padStart(2, '0')
  // 放大镜浮在光标上方（不盖住取的点），贴近顶边时翻到下方
  const px = e.clientX - rect.left, py = e.clientY - rect.top
  let ly = py - LOUPE / 2 - 24
  if (ly - LOUPE / 2 < 4) ly = py + LOUPE / 2 + 24
  const lx = Math.max(LOUPE / 2 + 4, Math.min(px, rect.width - LOUPE / 2 - 4))
  hoverPos.value = { x: px, y: py, lx, ly, show: true }
  drawLoupe(canvas, x, y)
  return `#${to(d[0])}${to(d[1])}${to(d[2])}`
}

// 放大镜：取光标周围 LOUPE_N×LOUPE_N 源像素放大铺满圆窗，中心格描边
function drawLoupe(src: HTMLCanvasElement, cx: number, cy: number) {
  const l = loupeEl.value
  if (!l) return
  const ctx = l.getContext('2d')
  if (!ctx) return
  const cell = LOUPE / LOUPE_N
  const half = Math.floor(LOUPE_N / 2)
  ctx.imageSmoothingEnabled = false
  ctx.clearRect(0, 0, LOUPE, LOUPE)
  ctx.drawImage(src, cx - half, cy - half, LOUPE_N, LOUPE_N, 0, 0, LOUPE, LOUPE)
  // 中心像素格：黑+白双描边，深浅底色上都看得清
  const o = half * cell
  ctx.strokeStyle = 'rgba(255,255,255,0.9)'
  ctx.lineWidth = 4
  ctx.strokeRect(o, o, cell, cell)
  ctx.strokeStyle = '#1c1b1a'
  ctx.lineWidth = 2
  ctx.strokeRect(o, o, cell, cell)
}

function onImgMove(e: PointerEvent) {
  const c = sampleAt(e)
  if (!c) { hoverPos.value.show = false; return }
  hoverColor.value = c
  if (pressing) void pick(c, true)
}
function onImgLeave() { hoverPos.value.show = false }
function onImgDown(e: PointerEvent) {
  pressing = true
  const c = sampleAt(e)
  if (c) void pick(c)
}
function onImgUp() {
  if (pressing && picked.value) message.success(`${picked.value.toUpperCase()} 已复制`)
  pressing = false
}

// 主色提取：缩略采样 + 16 级色桶计数
function extractPalette(ctx: CanvasRenderingContext2D, w: number, h: number) {
  const data = ctx.getImageData(0, 0, w, h).data
  const buckets = new Map<string, { r: number; g: number; b: number; n: number }>()
  const step = Math.max(1, Math.floor((w * h) / 8000)) * 4
  for (let i = 0; i < data.length; i += step) {
    const r = data[i] >> 4, g = data[i + 1] >> 4, b = data[i + 2] >> 4
    const key = `${r},${g},${b}`
    const cur = buckets.get(key) ?? { r: 0, g: 0, b: 0, n: 0 }
    cur.r += data[i]; cur.g += data[i + 1]; cur.b += data[i + 2]; cur.n++
    buckets.set(key, cur)
  }
  const to = (v: number) => v.toString(16).padStart(2, '0')
  palette.value = [...buckets.values()]
    .sort((a, b) => b.n - a.n)
    .slice(0, 8)
    .map(b => `#${to(Math.round(b.r / b.n))}${to(Math.round(b.g / b.n))}${to(Math.round(b.b / b.n))}`)
}

async function pick(c: string, quiet = false) {
  picked.value = c
  if (quiet) return
  message.success(await copyText(c.toUpperCase()) ? `${c.toUpperCase()} 已复制` : '复制失败')
}

async function copyPicked() {
  if (picked.value) message.success(await copyText(picked.value.toUpperCase()) ? '已复制' : '失败')
}
</script>

<template>
  <div class="img-page">
    <div class="panel canvas-panel">
      <div class="panel-title">
        <n-icon style="vertical-align:-2px"><ImageOutline /></n-icon> 图片拾色
        <span class="hint">移动看颜色 · 点击取色 · 支持拖图进来</span>
        <span style="flex:1"></span>
        <n-button size="small" @click="openFile">
          <template #icon><n-icon><CloudUploadOutline /></n-icon></template>上传图片
        </n-button>
      </div>
      <input ref="fileInput" type="file" accept="image/*" style="display:none" @change="onFile" />
      <div
        class="canvas-zone" :class="{ empty: !hasImage }"
        @dragover.prevent @drop.prevent="onDrop"
      >
        <div v-if="hasImage" class="img-wrap">
          <canvas
            ref="canvasEl" class="img-canvas"
            @pointermove="onImgMove" @pointerdown="onImgDown" @pointerup="onImgUp" @pointerleave="onImgLeave"
          ></canvas>
          <div v-if="hoverPos.show" class="loupe" :style="{ left: hoverPos.lx + 'px', top: hoverPos.ly + 'px' }">
            <canvas ref="loupeEl" :width="LOUPE" :height="LOUPE" class="loupe-canvas"></canvas>
            <div class="loupe-hex">{{ hoverColor.toUpperCase() }}</div>
          </div>
          <div
            v-if="hoverPos.show" class="loupe-mark"
            :style="{ left: hoverPos.x + 'px', top: hoverPos.y + 'px', background: hoverColor }"
          ></div>
        </div>
        <div v-else class="img-empty" @click="openFile">
          <n-icon :size="40"><CloudUploadOutline /></n-icon>
          <div style="font-weight:800;margin-top:10px">上传或拖入一张图片</div>
          <div style="font-size:12px;color:var(--cb-ink-3);margin-top:4px">光标移动实时预览颜色，点击像素取色</div>
        </div>
      </div>
    </div>

    <div class="panel side-panel">
      <div class="panel-title">已取颜色</div>
      <div class="picked-swatch" :style="{ background: picked || '#f8f6f6' }" @click="copyPicked">
        <span class="swatch-hex">{{ picked ? picked.toUpperCase() : '尚未取色' }}</span>
      </div>

      <div class="panel-title" style="margin-top:18px">
        主色 <span v-if="palette.length" class="hint">{{ palette.length }} 色 · 点击复制</span>
      </div>
      <template v-if="palette.length">
        <div v-for="c in palette" :key="c" class="pal-cell" :style="{ background: c }" @click="pick(c)">
          <span>{{ c.slice(1).toUpperCase() }}</span>
        </div>
      </template>
      <div v-else class="pal-empty">上传图片后自动提取主色</div>
    </div>
  </div>
</template>

<style scoped>
.img-page { display: grid; grid-template-columns: 1fr 220px; gap: 14px; align-items: start; }
@media (max-width: 900px) { .img-page { grid-template-columns: 1fr; } }
.canvas-panel { display: flex; flex-direction: column; }
.canvas-zone { flex: 1; }
.canvas-zone.empty { display: flex; }
.img-wrap { position: relative; display: inline-block; max-width: 100%; }
.img-canvas {
  display: block; max-width: 100%; border: var(--cb-border); border-radius: 12px;
  cursor: crosshair; touch-action: none;
}
.loupe {
  position: absolute; transform: translate(-50%, -50%);
  width: 132px; height: 132px; border-radius: 50%; overflow: hidden;
  border: 3px solid var(--cb-ink); box-shadow: 4px 4px 0 var(--cb-ink);
  pointer-events: none;
}
.loupe-mark {
  position: absolute; width: 7px; height: 7px; transform: translate(-50%, -50%);
  border: 1.5px solid #fff; outline: 1.5px solid var(--cb-ink);
  pointer-events: none; box-sizing: content-box;
}
.loupe-canvas { width: 100%; height: 100%; display: block; image-rendering: pixelated; }
.loupe-hex {
  position: absolute; left: 50%; bottom: 6px; transform: translateX(-50%);
  background: #fff; border: 1.5px solid var(--cb-ink); border-radius: 6px;
  font-size: 10px; font-weight: 800; padding: 1px 6px; font-family: Consolas, monospace;
  white-space: nowrap;
}
.img-empty {
  flex: 1; border: 3px dashed var(--cb-ink); border-radius: 14px;
  padding: 60px 20px; text-align: center; cursor: pointer; color: var(--cb-ink-2);
}
.img-empty:hover { background: var(--cb-yellow); }

.side-panel { display: flex; flex-direction: column; gap: 8px; }
.picked-swatch {
  height: 90px; border: var(--cb-border); border-radius: 14px;
  display: flex; align-items: flex-end; justify-content: center;
  cursor: pointer; transition: background 0.15s ease;
}
.swatch-hex {
  background: #fff; border: 2px solid var(--cb-ink); border-radius: 8px;
  font-weight: 900; font-size: 13px; padding: 3px 10px; margin-bottom: 8px;
}
.pal-cell {
  height: 44px; border: 2.5px solid var(--cb-ink); border-radius: 8px;
  cursor: pointer; transition: transform 0.12s ease;
  display: flex; align-items: flex-end; justify-content: flex-end;
}
.pal-cell span {
  font-size: 9px; font-weight: 800; background: rgba(255,255,255,0.85);
  border-radius: 4px; padding: 0 3px; margin: 3px;
}
.pal-cell:hover { transform: translateY(-3px); }
.pal-empty { font-size: 12px; color: var(--cb-ink-3); font-weight: 600; }
</style>

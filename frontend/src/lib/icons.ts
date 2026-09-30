// 工具图标名 → ionicons5 组件映射。工具表 icon 字段存这里的 key；
// 新增图标时往 map 里加一行即可，缺省回退 ConstructOutline。
import type { Component } from 'vue'
import {
  ConstructOutline, CodeOutline, TimeOutline, ImageOutline, LinkOutline,
  DocumentTextOutline, ColorPaletteOutline, KeyOutline, CalculatorOutline,
  GlobeOutline, TerminalOutline, WifiOutline, QrCodeOutline, TextOutline,
  FileTrayOutline, BugOutline, ChatboxOutline, SearchOutline,
  BookmarkOutline, StarOutline, HeartOutline, HomeOutline, CartOutline,
  GameControllerOutline, MusicalNotesOutline, VideocamOutline, NewspaperOutline,
  LogoGithub, FolderOutline, CloudOutline
} from '@vicons/ionicons5'

const map: Record<string, Component> = {
  'construct-outline': ConstructOutline,
  'code-outline': CodeOutline,
  'time-outline': TimeOutline,
  'image-outline': ImageOutline,
  'link-outline': LinkOutline,
  'document-text-outline': DocumentTextOutline,
  'color-palette-outline': ColorPaletteOutline,
  'key-outline': KeyOutline,
  'calculator-outline': CalculatorOutline,
  'globe-outline': GlobeOutline,
  'terminal-outline': TerminalOutline,
  'wifi-outline': WifiOutline,
  'qr-code-outline': QrCodeOutline,
  'text-outline': TextOutline,
  'file-tray-outline': FileTrayOutline,
  'bug-outline': BugOutline,
  'chatbox-outline': ChatboxOutline,
  'search-outline': SearchOutline,
  'bookmark-outline': BookmarkOutline,
  'star-outline': StarOutline,
  'heart-outline': HeartOutline,
  'home-outline': HomeOutline,
  'cart-outline': CartOutline,
  'game-controller-outline': GameControllerOutline,
  'musical-notes-outline': MusicalNotesOutline,
  'videocam-outline': VideocamOutline,
  'newspaper-outline': NewspaperOutline,
  'logo-github': LogoGithub,
  'folder-outline': FolderOutline,
  'cloud-outline': CloudOutline
}

// Memphis 色板：图标块底色按 id 轮转
const colors = ['#fef08a', '#bae6fd', '#a7f3d0', '#ffc1cc', '#d9b3ff', '#ffd8b0']

export function toolIcon(name: string): Component {
  return map[name] ?? ConstructOutline
}

export function toolGradient(id: number): string {
  return colors[Math.abs(id) % colors.length]
}

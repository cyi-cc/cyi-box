<script setup lang="ts">
import { NConfigProvider, NMessageProvider, NDialogProvider, zhCN, dateZhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'

// Memphis / 新粗野主义：亮橙主色 + 大圆角，黑边与硬投影在 style.css 统一覆盖
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#ec5b13',
    primaryColorHover: '#f5712b',
    primaryColorPressed: '#d14e0e',
    primaryColorSuppl: '#ec5b13',
    successColor: '#00a544',
    warningColor: '#f59e0b',
    errorColor: '#e40014',
    borderRadius: '10px',
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
    fontWeight: '500'
  },
  Menu: {
    itemColorHover: 'transparent',
    itemColorActive: 'transparent',
    itemTextColor: '#57534d',
    itemTextColorHover: '#1a1a1a',
    itemTextColorActive: '#1a1a1a',
    itemIconColor: '#8a857e',
    itemIconColorHover: '#1a1a1a',
    itemIconColorActive: '#1a1a1a',
    groupTextColor: '#8a857e',
    itemHeight: '42px',
    borderRadius: '10px',
    itemTextColorActiveHover: '#1a1a1a'
  },
  Card: { borderRadius: '16px' },
  Tag: { fontWeight: '700' }
}
</script>

<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

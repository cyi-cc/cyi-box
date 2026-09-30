import { ref } from 'vue'
import { defineStore } from 'pinia'
import { client } from '../lib/api'
import type bookmarkView from '../api/bookmarkView'

// 个人书签：驱动侧边栏「我的书签」组与书签页
export const useBookmarksStore = defineStore('bookmarks', () => {
  const items = ref<bookmarkView[]>([])
  const loaded = ref(false)

  async function refresh() {
    const r = await client.bookmarkSvc.list()
    if (r.status === 0 && r.data) {
      items.value = r.data
      loaded.value = true
    }
  }

  return { items, loaded, refresh }
})

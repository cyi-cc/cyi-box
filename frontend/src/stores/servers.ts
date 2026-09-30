import { defineStore } from 'pinia'
import { ref } from 'vue'
import { client } from '../lib/api'
import type serverView from '../api/serverView'

export const useServersStore = defineStore('servers', () => {
  const items = ref<serverView[]>([])
  const loaded = ref(false)

  async function refresh() {
    const r = await client.serverSvc.list()
    if (r.status === 0 && r.data) {
      items.value = r.data
      loaded.value = true
    }
  }

  function find(id: number) {
    return items.value.find(s => s.id === id)
  }

  return { items, loaded, refresh, find }
})

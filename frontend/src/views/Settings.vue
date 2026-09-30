<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import { client } from '../lib/api'

const message = useMessage()

const pwd = ref({ oldPassword: '', newPassword: '', confirm: '' })
const pwdSaving = ref(false)

async function changePwd() {
  if (!pwd.value.oldPassword || !pwd.value.newPassword) {
    message.warning('请填写完整')
    return
  }
  if (pwd.value.newPassword.length < 6) {
    message.warning('新密码至少 6 位')
    return
  }
  if (pwd.value.newPassword !== pwd.value.confirm) {
    message.warning('两次输入的新密码不一致')
    return
  }
  pwdSaving.value = true
  try {
    const r = await client.authSvc.changePassword({
      oldPassword: pwd.value.oldPassword,
      newPassword: pwd.value.newPassword
    })
    if (r.status === 0) {
      message.success('密码已修改')
      pwd.value = { oldPassword: '', newPassword: '', confirm: '' }
    } else {
      message.error(r.msg || '修改失败')
    }
  } finally {
    pwdSaving.value = false
  }
}
</script>

<template>
  <div class="settings-page">
    <div class="panel">
      <div class="panel-title">修改密码</div>
      <n-form label-placement="top" style="max-width: 360px">
        <n-form-item label="旧密码">
          <n-input v-model:value="pwd.oldPassword" type="password" show-password-on="click" placeholder="当前密码" />
        </n-form-item>
        <n-form-item label="新密码">
          <n-input v-model:value="pwd.newPassword" type="password" show-password-on="click" placeholder="至少 6 位" />
        </n-form-item>
        <n-form-item label="确认新密码">
          <n-input v-model:value="pwd.confirm" type="password" show-password-on="click" placeholder="再输一遍" @keyup.enter="changePwd" />
        </n-form-item>
      </n-form>
      <n-button type="primary" :loading="pwdSaving" @click="changePwd">保存</n-button>
    </div>
  </div>
</template>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 16px; max-width: 640px; }
</style>

<script setup>
// 用户消息 — 右对齐纯文本（定稿原型：无气泡）；大段粘贴折叠为附件条。
defineProps({
  text: { type: String, default: '' },
  attachment: { type: Object, default: null }
});
</script>

<template>
  <div class="um">
    <div v-if="attachment" class="um__file">
      <div class="um__fileico mono">{{ attachment.ext || 'TXT' }}</div>
      <div class="um__filemeta">
        <div class="um__filename">{{ attachment.name }}</div>
        <div class="um__filesize">{{ attachment.sizeText }}</div>
      </div>
    </div>
    <div class="um__text">{{ text }}</div>
  </div>
</template>

<style scoped>
.um {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  padding: 16px 24px;
}
.um__file {
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--g200);
  border-radius: var(--radius-lg);
  padding: 10px 12px;
  background: #fff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  max-width: 60%;
}
.um__fileico {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: var(--red50);
  border: 1px solid var(--red100);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 9px;
  font-weight: 700;
  color: var(--red500);
  letter-spacing: -0.5px;
  flex-shrink: 0;
}
.um__filemeta {
  min-width: 0;
}
.um__filename {
  font-size: 13px;
  color: var(--g900);
  font-weight: 500;
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.um__filesize {
  font-size: 11px;
  color: var(--g400);
  margin-top: 2px;
}
.um__text {
  max-width: 65%;
  font-size: 15px;
  color: var(--g900);
  line-height: 1.65;
  text-align: right;
  white-space: pre-wrap;
  overflow-wrap: break-word;
}
</style>

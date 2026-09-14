<script setup>
// Agent 图标（Manus 方向，用户提供）— 替代原渐变球。
defineProps({
  size: { type: Number, default: 32 },
  big: { type: Boolean, default: false }
});
</script>

<template>
  <img
    src="../assets/agent_icon.png"
    class="agent"
    :class="{ 'agent--big': big }"
    :style="{ width: size + 'px', height: size + 'px' }"
    alt=""
    aria-hidden="true"
  />
</template>

<style scoped>
.agent {
  border-radius: 26%;
  object-fit: cover;
  flex-shrink: 0;
  display: block;
}
.agent--big {
  border-radius: 30%;
}
</style>

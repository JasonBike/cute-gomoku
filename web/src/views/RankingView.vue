<script setup lang="ts">
import { ref } from "vue";

const period = ref<"daily" | "weekly" | "total">("weekly");
</script>

<template>
  <section class="view page-view ranking-page active" aria-label="排行榜">
    <header class="simple-header">
      <div><span class="section-kicker">GOMOKU CLUB</span><h1>棋坛排行榜</h1></div>
      <span class="crown-mark">♛</span>
    </header>
    <div class="segment-control" role="tablist">
      <button :class="{ active: period === 'daily' }" @click="period = 'daily'">今日</button>
      <button :class="{ active: period === 'weekly' }" @click="period = 'weekly'">本周</button>
      <button :class="{ active: period === 'total' }" @click="period = 'total'">总榜</button>
    </div>
    <div class="podium">
      <div class="podium-user second"><span class="podium-rank">2</span><span class="big-avatar blue-bear">熊</span><strong>下棋小熊</strong><small>1,174</small></div>
      <div class="podium-user first"><span class="crown">♛</span><span class="big-avatar gold-cat">喵</span><strong>猫猫大师</strong><small>1,286</small></div>
      <div class="podium-user third"><span class="podium-rank">3</span><span class="big-avatar green-rabbit">兔</span><strong>青团兔</strong><small>1,069</small></div>
    </div>
    <div class="full-ranking">
      <div class="list-heading"><span>排名与棋手</span><span>本周积分</span></div>
      <div class="full-rank-row"><b>4</b><span class="avatar avatar-yellow">团</span><span><strong>糯米团</strong><small>铂金 I · 68% 胜率</small></span><em>998</em></div>
      <div class="full-rank-row"><b>5</b><span class="avatar avatar-blue">蓝</span><span><strong>蓝莓棋手</strong><small>铂金 II · 6 连胜</small></span><em>951</em></div>
      <div class="full-rank-row current"><b>18</b><span class="avatar avatar-small"><span class="avatar-face">•ᴗ•</span></span><span><strong>小桃子 <i>我</i></strong><small>黄金 III · 再赢 2 局升级</small></span><em>806</em></div>
      <div class="full-rank-row"><b>19</b><span class="avatar avatar-yellow">瓜</span><span><strong>西瓜汽水</strong><small>黄金 III · 54% 胜率</small></span><em>794</em></div>
    </div>
  </section>
</template>

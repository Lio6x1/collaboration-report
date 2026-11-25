<script setup lang="ts">
import { computed } from 'vue';
import { user, identityUnavailable } from '../account/session';
import { useSearch } from './useSearch';
import { useSearchSpaces } from './useSearchSpaces';
import { searchExcerpt, searchResultPath } from './rules';
import AppIcon from '../../components/AppIcon.vue';

const props = defineProps<{ initialSpace: string }>();
const { text, limit, space, state, ready, search, next, retry } = useSearch(
  () => props.initialSpace,
);
const {
  spaces,
  cursor,
  loading: directoryLoading,
  error: directoryError,
  load,
  spaceName,
} = useSearchSpaces();
const selectedOutsideDirectory = computed(
  () => space.value !== '' && !spaces.value.some((entry) => entry.id === space.value),
);
</script>

<template>
  <section class="panel search-panel" aria-label="正式版搜索">
    <div class="section-title">
      <div>
        <span class="eyebrow">查找与发现 / SEARCH</span>
        <h1>搜索报告</h1>
      </div>
      <a v-if="!user" href="/">登录或注册</a>
    </div>
    <p class="hint">搜索当前可阅读的正式版。打开结果时，会重新核验资格并读取当前正式内容。</p>
    <form class="search-form" @submit.prevent="search">
      <label class="search-text"
        >搜索词<input
          v-model="text"
          type="search"
          aria-label="搜索词"
          autocomplete="off"
          placeholder="标题、正文或章节标题"
      /></label>
      <label
        >搜索范围<select v-model="space" aria-label="搜索范围">
          <option value="">全部可阅读正式版</option>
          <option v-if="selectedOutsideDirectory" :value="space">当前空间 {{ space }}</option>
          <option v-for="entry in spaces" :key="entry.id" :value="entry.id">
            {{ spaceName(entry) }}
          </option>
        </select></label
      >
      <label
        >每页数量<select v-model.number="limit" aria-label="每页数量">
          <option v-for="size in [1, 10, 20, 50, 100]" :key="size" :value="size">{{ size }}</option>
        </select></label
      >
      <button
        class="primary"
        :disabled="!ready || state?.loading"
        :aria-busy="state?.loading"
        type="submit"
      >
        <AppIcon name="search" />{{ state?.loading ? '搜索中…' : '搜索' }}
      </button>
    </form>
    <div v-if="user" class="search-directory">
      <button v-if="cursor" :disabled="directoryLoading || !ready" @click="load(true)">
        更多搜索空间
      </button>
      <p v-if="directoryLoading" class="hint" role="status">正在读取空间目录…</p>
      <p v-if="directoryError" class="error" role="alert">
        空间目录读取失败：{{ directoryError }}
        <button :disabled="directoryLoading || !ready" @click="load()">刷新空间目录</button>
      </p>
    </div>
    <p v-if="identityUnavailable" class="pending" role="status">
      当前资格待重新核验，旧搜索结果已清空。
    </p>
    <section class="search-results" aria-label="搜索结果" :aria-busy="state?.loading">
      <p v-if="state?.loading" role="status">正在核验完整搜索页…</p>
      <div v-if="state?.error" class="search-error">
        <p class="error" role="alert">{{ state.error }}</p>
        <button v-if="state.retryable" :disabled="!ready || state.loading" @click="retry">
          重试本次查询
        </button>
        <button v-else :disabled="!ready || state.loading" @click="search">重新搜索</button>
      </div>
      <div v-if="state && !state.loading && !state.error && !state.searched" class="empty-state">
        <span class="empty-icon"><AppIcon name="search" /></span>
        <h3>在正式内容中寻找答案</h3>
        <p>输入标题、正文或章节中的关键词后开始查询。</p>
      </div>
      <div
        v-if="state?.searched && !state.loading && !state.error && !state.items.length"
        class="empty-state"
        role="status"
      >
        <span class="empty-icon"><AppIcon name="search" /></span>
        <h3>没有当前可阅读的正式版结果。</h3>
        <p>试试其他关键词，或调整搜索范围。</p>
      </div>
      <ol v-if="state?.items.length" class="search-list">
        <li
          v-for="item in state.items"
          :key="item.document_id"
          :data-document-id="item.document_id"
        >
          <!-- 标题/摘要来自Doc最终授权输出，仅用文本插值；不解释HTML、不读取MAIN补内容。 -->
          <h2>
            <a :href="searchResultPath(item)">{{ item.title }}</a>
          </h2>
          <p class="search-snippet">{{ searchExcerpt(item.snippet, text) }}</p>
          <!-- 同名文档可以来自不同工作区；展示稳定编号，不能按标题合并合法结果。 -->
          <small class="search-identity">文档编号 {{ item.document_id }} · </small>
          <small>{{ item.chapter_id ? '打开当前正式版并定位章节' : '打开当前正式版' }}</small>
        </li>
      </ol>
      <div v-if="state?.page" class="search-pagination">
        <span role="status">第 {{ state.page }} 页 · 本页 {{ state.items.length }} 项</span>
        <button v-if="state.nextCursor" :disabled="!ready || state.loading" @click="next">
          下一页
        </button>
        <span v-else class="hint">已到最后一页</span>
      </div>
    </section>
    <p class="hint">翻页有固定期限；会话、查询范围或正式版变化后，请重新搜索。</p>
  </section>
</template>

<style scoped>
.search-panel h1 {
  font-size: clamp(30px, 3vw, 38px);
}
.search-panel {
  padding: 36px;
}
.search-form {
  display: grid;
  grid-template-columns: minmax(200px, 1fr) minmax(160px, 220px) 110px auto;
  align-items: end;
  gap: 14px;
  padding: 22px;
  margin: 24px 0 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #eeefe6;
}
.search-form label {
  min-width: 0;
}
.search-form input,
.search-form select {
  width: 100%;
}
.search-directory {
  margin-top: 10px;
}
.search-results {
  margin-top: 28px;
}
.search-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.search-list li {
  position: relative;
  padding: 24px 18px 24px 25px;
  border-bottom: 1px solid var(--line);
  border-left: 2px solid transparent;
  overflow-wrap: anywhere;
  transition:
    background 160ms ease,
    border-color 160ms ease;
}
.search-list li:hover {
  background: #f3f5eb;
  border-left-color: #8c9d67;
}
.search-list h2 {
  margin-bottom: 8px;
  font-size: var(--ui-font-title);
}
.search-list a {
  color: var(--accent);
  text-underline-offset: 3px;
}
.search-snippet {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
  white-space: normal;
  line-height: 1.6;
  max-height: 4.8em;
  color: var(--muted);
}
.search-list small {
  color: var(--muted);
  font-size: var(--ui-font-meta);
}
.search-pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-top: 22px;
}
@media (max-width: 900px) {
  .search-form {
    grid-template-columns: 1fr 1fr;
  }
  .search-text {
    grid-column: 1 / -1;
  }
}
@media (max-width: 520px) {
  .search-panel {
    padding: 24px 18px;
  }
  .search-form {
    grid-template-columns: 1fr;
    padding: 18px 14px;
  }
  .search-list li {
    padding: 22px 10px 22px 14px;
  }
  .search-text {
    grid-column: auto;
  }
  .search-pagination {
    align-items: start;
    flex-direction: column;
  }
}
</style>

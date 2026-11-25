<script setup lang="ts">
import { computed } from 'vue';
import type { PublicDocument } from '../../../../contracts/http/generated';
import ContentView from '../../lib/content/ContentView.vue';
import { chapterAnchor } from './anchors';
import { chapterGroups, unitTitle } from '../structure/reading';
const props = defineProps<{ publication: PublicDocument; label: string }>();
const units = computed(() => new Map(props.publication.units.map((unit) => [unit.unit_id, unit])));
const anchor = (unit: string) => 'release-' + props.publication.release_id + '-unit-' + unit;
const chapters = computed(() => chapterGroups(props.publication.structure.entries));
</script>
<template>
  <div class="reading-layout formal-reading" :data-current-release="publication.release_id">
    <nav aria-label="正式版目录">
      <h3>目录</h3>
      <ol>
        <li v-for="group in chapters" :key="group.chapter.node_id">
          <a :href="'#' + chapterAnchor(group.chapter.chapter_id)">{{ group.chapter.title }}</a>
          <ol v-if="group.chapter.split_level2">
            <li v-for="entry in group.units" :key="entry.node_id">
              <a :href="'#' + anchor(entry.unit_id!)">{{ unitTitle(entry) }}</a>
            </li>
          </ol>
        </li>
      </ol>
      <p class="hint">
        A4 · {{ publication.style.style.body_font === 'SimSun' ? '宋体' : '仿宋' }}
      </p>
    </nav>
    <article
      class="paper"
      :aria-label="label + '整篇'"
      :style="{
        fontFamily: publication.style.style.body_font + ', Times New Roman, serif',
        fontSize: publication.style.style.body_size_pt + 'pt',
      }"
    >
      <h1>{{ publication.title }}</h1>
      <section
        v-for="group in chapters"
        :id="chapterAnchor(group.chapter.chapter_id)"
        :key="group.chapter.node_id"
        :aria-label="'章节：' + group.chapter.title"
        class="chapter-anchor"
      >
        <h3>{{ group.chapter.title }}</h3>
        <div
          v-for="entry in group.units"
          :id="anchor(entry.unit_id!)"
          :key="entry.node_id"
          class="publication-unit"
        >
          <h4 v-if="entry.kind === 'SECTION'" class="publication-section-title">
            {{ entry.title }}
          </h4>
          <!-- 只传入release自己的完整不可变正文和样式，不建立编辑连接或读取后来MAIN。 -->
          <ContentView
            v-if="units.get(entry.unit_id!)"
            :content="units.get(entry.unit_id!)!"
            :style="publication.style"
            :resource-context="{
              document: publication.document_id,
              kind: 'PUBLIC',
              id: publication.release_id,
            }"
            :label="label"
          />
        </div>
      </section>
    </article>
  </div>
</template>
<style scoped>
.formal-reading .paper {
  overflow-wrap: anywhere;
}
.formal-reading h1 {
  margin-bottom: 32px;
  font-size: 28px;
}
.formal-reading nav ol {
  display: block;
}
.publication-unit + .publication-unit {
  margin-top: 2rem;
}
.publication-section-title {
  font-size: 1.3em;
  margin: 0 0 1rem;
}
.chapter-anchor {
  display: block;
  scroll-margin-top: 24px;
}
@media (max-width: 760px) {
  .formal-reading {
    grid-template-columns: 1fr;
  }
}
</style>

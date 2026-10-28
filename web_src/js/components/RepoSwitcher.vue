<script>
import {nextTick} from 'vue';
import {SvgIcon} from '../svg.ts';
import {showErrorToast} from '../modules/toast.ts';
import {GET} from '../modules/fetch.ts';
import {filterRepos} from '../features/repo-switcher.ts';
export default {
  components: {SvgIcon},

  // data() is provided by the init code in features/repo-switcher.ts

  computed: {
    filteredItems() {
      return filterRepos(this.repos, this.searchTerm);
    },
    showNoResults() {
      return !this.isLoading && !this.loadError && this.loaded && !this.filteredItems.length;
    },
  },

  watch: {
    menuVisible(visible) {
      if (visible) {
        this.active = -1;
        this.focusSearchField();
        this.fetchRepos();
      }
    },
    searchTerm() {
      this.active = -1;
    },
  },

  beforeMount() {
    document.body.addEventListener('click', (event) => {
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) this.menuVisible = false;
    });
  },

  methods: {
    toggleMenu() {
      this.menuVisible = !this.menuVisible;
    },
    focusSearchField() {
      nextTick(() => {
        this.$refs.searchField?.focus();
      });
    },
    selectItem(item) {
      window.location.href = item.link;
    },
    keydown(event) {
      if (event.keyCode === 40) { // arrow down
        event.preventDefault();
        if (this.active + 1 < this.filteredItems.length) this.active++;
      } else if (event.keyCode === 38) { // arrow up
        event.preventDefault();
        if (this.active > 0) this.active--;
      } else if (event.keyCode === 13) { // enter
        event.preventDefault();
        if (this.active >= 0 && this.active < this.filteredItems.length) {
          this.selectItem(this.filteredItems[this.active]);
        }
      } else if (event.keyCode === 27) { // escape
        event.preventDefault();
        this.menuVisible = false;
      }
    },
    async fetchRepos() {
      if (this.loaded || this.isLoading) return;
      this.isLoading = true;
      this.loadError = false;
      try {
        const resp = await GET(this.fetchUrl);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const {results, more} = await resp.json();
        this.repos = results;
        this.more = Boolean(more);
        this.loaded = true;
      } catch (e) {
        this.loadError = true;
        showErrorToast(`${this.textLoadError}: ${e}`);
      } finally {
        this.isLoading = false;
      }
    },
  },
};
</script>
<template>
  <div class="ui dropdown custom repo-switcher">
    <button
      class="repo-switcher-button"
      :aria-label="textLabel"
      :aria-expanded="menuVisible ? 'true' : 'false'"
      :data-tooltip-content="textLabel"
      @click="toggleMenu"
    >
      <svg-icon name="octicon-triangle-down" :size="14"/>
    </button>
    <div class="menu transition" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak>
      <div class="ui icon search input">
        <i class="icon"><svg-icon name="octicon-filter" :size="16"/></i>
        <input
          name="search" ref="searchField" autocomplete="off" v-model="searchTerm"
          @keydown="keydown($event)" :placeholder="textSearchPlaceholder"
        >
      </div>
      <div class="scrolling menu" ref="scrollContainer">
        <div class="loading-indicator is-loading" v-if="isLoading"/>
        <a
          v-for="(item, index) in filteredItems"
          :key="item.full_name"
          class="item"
          :class="{selected: item.name === currentRepoName, active: active === index}"
          :href="item.link"
          @click.prevent="selectItem(item)"
        >
          <svg-icon :name="item.private ? 'octicon-lock' : (item.archived ? 'octicon-archive' : 'octicon-repo')" :size="16"/>
          <span class="gt-ellipsis tw-ml-2">{{ item.name }}</span>
        </a>
      </div>
      <div class="message" v-if="showNoResults">
        {{ textNoResults }}
      </div>
      <a class="item repo-switcher-search-all" v-if="more && loaded" :href="ownerReposUrl">
        <svg-icon name="octicon-search" :size="16"/>
        <span class="tw-ml-2">{{ textSearchAll }}</span>
      </a>
    </div>
  </div>
</template>
<style scoped>
.repo-switcher .scrolling.menu {
  max-height: 20rem;
}
.repo-switcher .item.selected {
  font-weight: var(--font-weight-semibold);
}
.repo-switcher .loading-indicator {
  height: 4em;
}
</style>

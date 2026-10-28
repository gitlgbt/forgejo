import {createApp} from 'vue';

export type RepoSwitcherItem = {
  name: string;
  full_name: string;
  link: string;
  private: boolean;
  archived: boolean;
};

export function filterRepos(repos: RepoSwitcherItem[], term: string): RepoSwitcherItem[] {
  const needle = (term || '').trim().toLowerCase();
  if (!needle) return repos;
  return repos.filter((repo) => repo.name.toLowerCase().includes(needle));
}

type RepoSwitcherData = {
  currentRepoName: string;
  fetchUrl: string;
  ownerReposUrl: string;
  textLabel: string;
  textSearchPlaceholder: string;
  textNoResults: string;
  textSearchAll: string;
  textLoadError: string;
};

export async function initRepoSwitcher() {
  const dataList = (window.config?.pageData?.repoSwitcherDataList ?? []) as RepoSwitcherData[];
  const els = document.querySelectorAll('.js-repo-switcher');
  for (const [elIndex, elRoot] of els.entries()) {
    const {default: RepoSwitcher} = await import(/* webpackChunkName: "repo-switcher" */ '../components/RepoSwitcher.vue');
    createApp({
      ...RepoSwitcher,
      data() {
        return {
          repos: [],
          searchTerm: '',
          menuVisible: false,
          isLoading: false,
          loaded: false,
          loadError: false,
          more: false,
          active: -1,
          ...dataList[elIndex],
        };
      },
    }).mount(elRoot);
  }
}

import {createApp} from 'vue';
import {translateMonth, translateDay} from '../utils.ts';
import {GET} from '../modules/fetch.ts';

let heatmapHooksInstalled = false;
const heatmapSwapSelector = '.user-activity-layout';

function initHeatmapDropdowns(root) {
  const dropdowns = root.querySelectorAll('.ui.dropdown');
  if (!dropdowns.length) return true;
  const $ = window.jQuery || window.$;
  if (!$ || !$.fn || !$.fn.dropdown) return false;
  $(dropdowns).dropdown();
  return true;
}

async function swapHeatmapContent(url) {
  const current = document.querySelector(heatmapSwapSelector);
  if (!current) return false;
  try {
    const resp = await GET(url);
    if (!resp.ok) return false;
    const doc = new DOMParser().parseFromString(await resp.text(), 'text/html');
    const next = doc.querySelector(heatmapSwapSelector);
    if (!next) return false;
    current.replaceWith(next);
    window.history.pushState({}, '', url);
    if (!initHeatmapDropdowns(next)) return false;
    initHeatmap();
    return true;
  } catch (err) {
    console.error('Heatmap swap failed', err);
    return false;
  }
}

function installHeatmapHooks() {
  if (heatmapHooksInstalled) return;
  heatmapHooksInstalled = true;

  document.addEventListener('click', (e) => {
    const link = e.target.closest?.('.user-heatmap-years a, .user-heatmap-orgs a');
    if (!link || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    const url = link.href;
    swapHeatmapContent(url).then((ok) => {
      if (!ok) window.location.href = url;
    });
  });

  window.addEventListener('popstate', () => {
    window.location.reload();
  });
}

export async function initHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;
  installHeatmapHooks();
  if (!el.classList.contains('is-loading')) return; 

  const {default: ActivityHeatmap} = await import(/* webpackChunkName: "activity-heatmap" */'../components/ActivityHeatmap.vue');
  try {
    const heatmap = {};
    for (const {contributions, timestamp} of JSON.parse(el.getAttribute('data-heatmap-data'))) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }

    const values = Object.keys(heatmap).map((v) => {
      return {date: new Date(v), count: heatmap[v]};
    });

    const locale = {
      months: new Array(12).fill().map((_, idx) => translateMonth(idx)),
      days: new Array(7).fill().map((_, idx) => translateDay(idx)),
      contributions_in_the_last_12_months: el.getAttribute('data-locale-total-contributions'),
      contributions_zero: el.getAttribute('data-locale-contributions-zero'),
      contributions_format: el.getAttribute('data-locale-contributions-format'),
      contributions_one: el.getAttribute('data-locale-contributions-one'),
      contributions_few: el.getAttribute('data-locale-contributions-few'),
      more: el.getAttribute('data-locale-more'),
      less: el.getAttribute('data-locale-less'),
    };

    const endAttr = el.getAttribute('data-end-date');
    const forcedEndDate = endAttr ? new Date(`${endAttr}T23:59:59`) : null;
    const View = createApp(ActivityHeatmap, {values, locale, forcedEndDate});
    View.mount(el);
    el.classList.remove('is-loading');
  } catch (err) {
    console.error('Heatmap failed to load', err);
    el.textContent = 'Heatmap failed to load';
  }
}

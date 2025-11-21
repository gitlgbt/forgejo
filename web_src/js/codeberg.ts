import {showElem, hideElem} from "./utils/dom";

export function initCodeberg() {
  if (localStorage.getItem('tou-announcement-2') === 'dismissed') {
    return;
  }

  showElem('#tou-announcement');
  document.querySelector('#tou-announcement button')?.addEventListener('click', () => {
    localStorage.setItem('tou-announcement-2', 'dismissed');
    hideElem('#tou-announcement');
  }, {passive: true, once: true});
}

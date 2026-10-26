// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {beforeEach, expect, test, vi} from 'vitest';
import {initAnnouncement} from './announcement.js';

function render(id) {
  document.body.innerHTML = `<aside class="gitlgbt-announcement" data-announcement-id="${id}"><button class="gitlgbt-announcement-dismiss" hidden>Dismiss</button></aside>`;
  initAnnouncement();
}

beforeEach(() => {
  window.localStorage.clear();
  vi.restoreAllMocks();
});

test('remembers dismissal across navigation but shows a new announcement', () => {
  render('first');
  expect(document.querySelector('button').hidden).toBe(false);
  document.querySelector('button').click();
  expect(document.querySelector('aside')).toBeNull();
  render('first');
  expect(document.querySelector('aside')).toBeNull();
  render('second');
  expect(document.querySelector('aside')).not.toBeNull();
});

test('can still dismiss when browser storage is blocked', () => {
  vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => { throw new Error('blocked') });
  vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => { throw new Error('blocked') });
  render('first');
  document.querySelector('button').click();
  expect(document.querySelector('aside')).toBeNull();
});

test('dismisses multiple announcements independently', () => {
  document.body.innerHTML = ['first', 'second'].map((id) => `<aside class="gitlgbt-announcement" data-announcement-id="${id}"><button class="gitlgbt-announcement-dismiss" hidden>Dismiss</button></aside>`).join('');
  initAnnouncement();
  document.querySelector('button').click();
  expect(document.querySelectorAll('aside')).toHaveLength(1);
  expect(document.querySelector('aside').dataset.announcementId).toBe('second');
  document.querySelector('button').click();
  render('first');
  expect(document.querySelector('aside')).toBeNull();
  render('second');
  expect(document.querySelector('aside')).toBeNull();
});

test('ignores buttons inside rendered Markdown', () => {
  document.body.innerHTML = '<aside class="gitlgbt-announcement" data-announcement-id="one"><div><button type="button">Content</button></div><button class="gitlgbt-announcement-dismiss" hidden>Dismiss</button></aside>';
  initAnnouncement();
  document.querySelector('div button').click();
  expect(document.querySelector('aside')).not.toBeNull();
  expect(window.localStorage.length).toBe(0);
  const dismiss = document.querySelector('.gitlgbt-announcement-dismiss');
  expect(dismiss.hidden).toBe(false);
  dismiss.click();
  expect(document.querySelector('aside')).toBeNull();
});

// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

export function initAnnouncement() {
  for (const banner of document.querySelectorAll('.gitlgbt-announcement')) {
    initBanner(banner);
  }
}

function initBanner(banner) {
  const {announcementId} = banner.dataset;
  const key = `gitlgbt-announcement:${window.config.appSubUrl}:${announcementId}`;
  try {
    if (window.localStorage.getItem(key) === announcementId) {
      banner.remove();
      return;
    }
  } catch {
    // Storage can be unavailable in private browsing.
  }
  const dismiss = banner.querySelector(':scope > .gitlgbt-announcement-dismiss');
  dismiss.hidden = false;
  dismiss.addEventListener('click', () => {
    try {
      window.localStorage.setItem(key, announcementId);
    } catch {
      // Dismiss for this page even when storage is unavailable.
    }
    banner.remove();
  });
}

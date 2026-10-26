// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {initAnnouncement} from './announcement.js';
import {initAnnouncementEditors} from './announcement-editor.js';

function init() {
  initAnnouncement();
  initAnnouncementEditors();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init, {once: true});
} else {
  init();
}

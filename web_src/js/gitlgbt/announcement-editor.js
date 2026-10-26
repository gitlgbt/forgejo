// Copyright 2026 The Codefloe Authors. All rights reserved.
// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

import {initComboMarkdownEditor} from '../features/comp/ComboMarkdownEditor.js';

export function initAnnouncementEditors() {
  for (const container of document.querySelectorAll('.gitlgbt-announcement-editor')) {
    const textarea = container.querySelector('textarea');
    textarea.maxLength = 500;
    initComboMarkdownEditor(container, {
      previewMode: 'gfm',
      useScene: 'announcement',
      editorHeights: {minHeight: '120px'},
    });
  }
}

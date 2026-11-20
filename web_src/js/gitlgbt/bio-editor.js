// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {initComboMarkdownEditor} from '../features/comp/ComboMarkdownEditor.js';

export function initBioEditors() {
  for (const container of document.querySelectorAll('.gitlgbt-bio-editor')) {
    const textarea = container.querySelector('textarea');
    textarea.maxLength = 255;
    initComboMarkdownEditor(container, {
      previewMode: 'gfm',
      editorHeights: {minHeight: '120px'},
    });
  }
}
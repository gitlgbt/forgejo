// Copyright 2026 The Fryrvo and Gitlgbt Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later
document.addEventListener('click', (e) => {
  if (!window.matchMedia('(hover: none)').matches) return;
  const badge = e.target.closest('.org-badge');
  for (const open of document.querySelectorAll('.org-badge.is-open')) {
    if (open !== badge) open.classList.remove('is-open');
  }
  if (!badge) return;
  if (!badge.classList.contains('is-open')) {
    if (badge.tagName === 'A') e.preventDefault();
    badge.classList.add('is-open');
  } else if (badge.tagName !== 'A') {
    badge.classList.remove('is-open');
  }
}, true);

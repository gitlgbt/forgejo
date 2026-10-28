import {filterRepos} from './repo-switcher.ts';

const repos = [
  {name: 'ricochet', full_name: 'ricochet-rs/ricochet', link: '/ricochet-rs/ricochet', private: false, archived: false},
  {name: 'Ricochet-docs', full_name: 'ricochet-rs/Ricochet-docs', link: '/ricochet-rs/Ricochet-docs', private: false, archived: false},
  {name: 'website', full_name: 'ricochet-rs/website', link: '/ricochet-rs/website', private: true, archived: false},
];

test('empty term returns all repos', () => {
  expect(filterRepos(repos, '')).toHaveLength(3);
  expect(filterRepos(repos, '   ')).toHaveLength(3);
});

test('filters by case-insensitive substring of name', () => {
  expect(filterRepos(repos, 'rico').map((r) => r.name)).toEqual(['ricochet', 'Ricochet-docs']);
  expect(filterRepos(repos, 'WEB')).toHaveLength(1);
});

test('no match returns empty array', () => {
  expect(filterRepos(repos, 'zzz')).toEqual([]);
});

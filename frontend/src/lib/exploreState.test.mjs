import test from 'node:test';
import assert from 'node:assert/strict';
import { categorySelection, classifyExploreError, selectedCategoryAfterRefresh } from './exploreState.js';

test('keeps only a selectable category from the refreshed catalog', () => {
  const entries = [{ id: 'new-control', selectable: false }, { id: 'new-category', selectable: true }];
  assert.equal(selectedCategoryAfterRefresh('new-category', entries), 'new-category');
  assert.equal(selectedCategoryAfterRefresh('old-category', entries), '');
  assert.equal(selectedCategoryAfterRefresh('new-control', entries), '');
});

test('restores cached pages when revisiting a category and ignores active reselection', () => {
  const saved = { results: [{ name: 'page 1' }, { name: 'page 2' }], nextPage: 3, exhausted: false };
  const cache = { A: saved };
  assert.deepEqual(categorySelection('B', 'A', cache), { kind: 'cached', state: saved });
  assert.deepEqual(categorySelection('A', 'A', cache), { kind: 'current' });
  assert.deepEqual(categorySelection('A', 'B', cache), { kind: 'load' });
});

test('classifies session, page conflict, retryable, and terminal failures', () => {
  assert.deepEqual(classifyExploreError({ code: 'session_not_found' }), { kind: 'reopen' });
  assert.deepEqual(classifyExploreError({ code: 'page_conflict', nextPage: 3 }), { kind: 'page', page: 3 });
  assert.deepEqual(classifyExploreError({ code: 'transport_failed', retryable: true }), { kind: 'retry' });
  assert.deepEqual(classifyExploreError({ code: 'invalid_control_value' }), { kind: 'stop' });
});

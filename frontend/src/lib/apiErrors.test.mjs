import test from 'node:test';
import assert from 'node:assert/strict';
import { isExploreErrorBody } from './apiErrors.mjs';

test('only structured Explore diagnostics use ExploreApiError', () => {
  assert.equal(isExploreErrorBody({
    code: 'transport_failed', stage: 'transport', severity: 'error',
    message: 'Explore request failed', retryable: true,
  }), true);
  assert.equal(isExploreErrorBody({
    error: 'book info fetch failed', code: 'book_info_failed', workflow: 'book_info',
  }), false);
  assert.equal(isExploreErrorBody({ error: 'reader storage unavailable', code: 'storage_failed' }), false);
});

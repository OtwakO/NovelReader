import { describe, expect, it } from 'vitest';
import { isExploreErrorBody } from './errors';

describe('isExploreErrorBody', () => {
  it('requires the complete structured explore error shape', () => {
    expect(isExploreErrorBody({ code: 'x', stage: 'catalog', severity: 'error', message: 'failed' })).toBe(true);
    expect(isExploreErrorBody({ code: 'x', message: 'failed' })).toBe(false);
  });
});

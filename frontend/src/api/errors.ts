export interface ExploreErrorBodyLike {
  code?: unknown;
  stage?: unknown;
  severity?: unknown;
  message?: unknown;
}

export function isExploreErrorBody(body: unknown): body is Required<ExploreErrorBodyLike> {
  if (!body || typeof body !== 'object') return false;
  const value = body as ExploreErrorBodyLike;
  return typeof value.code === 'string' &&
    typeof value.stage === 'string' &&
    typeof value.severity === 'string' &&
    typeof value.message === 'string';
}

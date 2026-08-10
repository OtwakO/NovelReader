export function isExploreErrorBody(body) {
  return Boolean(
    body &&
    typeof body === 'object' &&
    typeof body.code === 'string' &&
    typeof body.stage === 'string' &&
    typeof body.severity === 'string' &&
    typeof body.message === 'string'
  );
}

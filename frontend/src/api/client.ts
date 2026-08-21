// Compatibility barrel during the Vue vertical-slice migration.
// New feature modules should import the narrow domain client directly.
export * from './transport';
export * from './auth';
export * from './sources';
export * from './search';
export * from './explore';
export * from './books';
export * from './reader';
export * from './system';

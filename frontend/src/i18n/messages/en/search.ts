export default {
  title: 'Search', description: 'Search enabled sources by title or author. Results appear progressively and equivalent books are consolidated without hiding their available sources.',
  form: { label: 'Book title or author', placeholder: 'Search by title or author…' },
  actions: { search: 'Search', stop: 'Stop', retry: 'Retry this batch', restart: 'Restart search', more: 'Scan {count} more sources' },
  controls: { title: 'Search coverage and intensity', batchSize: 'Sources per batch', intensity: 'Search intensity', gentle: 'Gentle · 4 concurrent', balanced: 'Balanced · 8 concurrent', fast: 'Fast · 16 concurrent', advanced: 'Advanced', concurrency: 'Concurrency' },
  status: { checkedOf: '{checked} of {eligible} sources checked', checked: '{checked} sources checked', results: '{count} books found', concurrency: 'Concurrency {count}', failures: '{count} source failures in completed batches', disconnected: 'The search connection was interrupted. Retry this batch to continue without losing results.', stale: 'The source list changed while searching. Restart the search.', storage: 'Search state could not be saved in this tab.' },
  results: { label: 'Search results', summary: '{count} consolidated books', multiple: '{count} have multiple sources', detailsFor: 'View details for {name}', sources: '{count} sources', details: 'Details', shelve: 'Add to shelf', shelving: 'Adding…', added: 'Added to your shelf.', addFailed: 'This book could not be added to the shelf.' },
  empty: { title: 'No books found', description: 'Try another title or author, continue scanning if more sources are available, or check that searchable sources are enabled.' },
};

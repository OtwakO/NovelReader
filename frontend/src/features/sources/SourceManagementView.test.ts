import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SourceManagementView from './SourceManagementView.vue';

const listSources = vi.fn();
const listSourceCollections = vi.fn();
const getSource = vi.fn();
const updateSourcePreferences = vi.fn();
const updateSourceCollection = vi.fn();
const loadExploreSources = vi.fn();

vi.mock('../../api/sources', () => ({
  listSources: (...args: unknown[]) => listSources(...args),
  getSource: (...args: unknown[]) => getSource(...args),
  importSources: vi.fn(),
  updateSource: vi.fn(),
  updateSourcePreferences: (...args: unknown[]) => updateSourcePreferences(...args),
  deleteSource: vi.fn(),
}));
vi.mock('../../api/source-collections', () => ({
  listSourceCollections: (...args: unknown[]) => listSourceCollections(...args),
  updateSourceCollection: (...args: unknown[]) => updateSourceCollection(...args),
  createUploadCollection: vi.fn(),
  createURLCollection: vi.fn(),
  deleteSourceCollection: vi.fn(),
  replaceUploadCollection: vi.fn(),
  syncSourceCollection: vi.fn(),
}));
vi.mock('../explore/explore-store', () => ({
  useExploreStore: () => ({ loadSources: loadExploreSources, refreshSource: vi.fn() }),
}));

describe('SourceManagementView collection availability', () => {
  beforeEach(() => {
    listSources.mockReset();
    listSourceCollections.mockReset();
    getSource.mockReset();
    updateSourcePreferences.mockReset();
    updateSourceCollection.mockReset();
    loadExploreSources.mockReset();
  });

  it('filters by the source setting without treating a paused collection as disabled', async () => {
    listSources.mockResolvedValue([
      {
        sourceId: 'enabled-source', collectionId: 'paused-collection', bookSourceUrl: 'https://enabled.test',
        bookSourceName: 'Enabled source', enabled: true, enabledExplore: true, exploreUrl: 'Books::/books', searchable: true, capabilities: ['search', 'explore'],
      },
      {
        sourceId: 'disabled-source', collectionId: '', bookSourceUrl: 'https://disabled.test',
        bookSourceName: 'Disabled source', enabled: false, enabledExplore: false, searchable: false, capabilities: [],
      },
    ]);
    listSourceCollections.mockResolvedValue([{
      id: 'paused-collection', name: 'Paused collection', originKind: 'upload', originFilename: 'sources.json',
      syncInterval: 'manual', enabled: false, sourceCount: 1, createdAt: 1, updatedAt: 1,
    }]);

    const wrapper = mount(SourceManagementView, {
      global: {
        mocks: { $t: (key: string) => key },
        stubs: {
          FeatureScaffold: { template: '<main><slot /></main>' },
          AppButton: { template: '<button><slot /></button>' },
          SourceCollectionDialog: true,
          SourceEditorDialog: true,
          SourceImportDialog: true,
          SourceInteractionSheet: true,
        },
      },
    });
    await flushPromises();

    wrapper.vm.enabledFilter = 'enabled';
    await wrapper.vm.$nextTick();
    expect(wrapper.findAll('.source-list li')).toHaveLength(1);
    expect(wrapper.get('.source-list li').text()).toContain('Enabled source');
    expect(wrapper.get('.source-list li').classes()).toContain('collection-disabled');

    wrapper.vm.enabledFilter = 'disabled';
    await wrapper.vm.$nextTick();
    expect(wrapper.findAll('.source-list li')).toHaveLength(1);
    expect(wrapper.get('.source-list li').text()).toContain('Disabled source');
  });

  it('patches a summary toggle without loading or replacing the definition', async () => {
    const summary = {
      sourceId: 'source-1', bookSourceUrl: 'https://source.test', bookSourceName: 'Source',
      enabled: true, enabledExplore: true, exploreUrl: 'Books::/books', searchable: true,
      capabilities: ['search', 'explore'],
    };
    listSources.mockResolvedValue([summary]);
    listSourceCollections.mockResolvedValue([]);
    updateSourcePreferences.mockResolvedValue({ ...summary, enabled: false });

    const wrapper = mount(SourceManagementView, {
      global: {
        mocks: { $t: (key: string) => key },
        stubs: {
          FeatureScaffold: { template: '<main><slot /></main>' },
          AppButton: { template: '<button><slot /></button>' },
          SourceCollectionDialog: true, SourceEditorDialog: true, SourceImportDialog: true, SourceInteractionSheet: true,
        },
      },
    });
    await flushPromises();
    await wrapper.get('.switches input').trigger('change');
    await flushPromises();

    expect(getSource).not.toHaveBeenCalled();
    expect(updateSourcePreferences).toHaveBeenCalledWith('source-1', { enabled: false });
  });

  it('pauses discovery without changing the member source setting', async () => {
    listSources.mockResolvedValue([{
      sourceId: 'source-1', collectionId: 'collection-1', bookSourceUrl: 'https://source.test',
      bookSourceName: 'Source', enabled: true, enabledExplore: true, exploreUrl: 'Books::/books', searchable: true, capabilities: ['search', 'explore'],
    }]);
    listSourceCollections.mockResolvedValue([{
      id: 'collection-1', name: 'Collection', originKind: 'upload', originFilename: 'sources.json',
      syncInterval: 'manual', enabled: true, sourceCount: 1, createdAt: 1, updatedAt: 1,
    }]);
    updateSourceCollection.mockResolvedValue({
      id: 'collection-1', name: 'Collection', originKind: 'upload', originFilename: 'sources.json',
      syncInterval: 'manual', enabled: false, sourceCount: 1, createdAt: 1, updatedAt: 2,
    });

    const wrapper = mount(SourceManagementView, {
      global: {
        mocks: { $t: (key: string) => key },
        stubs: {
          FeatureScaffold: { template: '<main><slot /></main>' },
          AppButton: { template: '<button><slot /></button>' },
          SourceCollectionDialog: true,
          SourceEditorDialog: true,
          SourceImportDialog: true,
          SourceInteractionSheet: true,
        },
      },
    });
    await flushPromises();
    const collectionButton = wrapper.findAll('.collection-strip button')[1];
    if (!collectionButton) throw new Error('collection button not rendered');
    await collectionButton.trigger('click');

    const availability = wrapper.get('.collection-availability input');
    const sourceEnabled = wrapper.get('.switches input');
    expect((sourceEnabled.element as HTMLInputElement).checked).toBe(true);

    await availability.trigger('change');
    await flushPromises();

    expect(updateSourceCollection).toHaveBeenCalledWith('collection-1', { enabled: false });
    expect(loadExploreSources).toHaveBeenCalledOnce();
    expect((sourceEnabled.element as HTMLInputElement).checked).toBe(true);
    expect(wrapper.get('.source-list li').classes()).toContain('collection-disabled');
    expect(wrapper.vm.selectedCollectionStats).toMatchObject({
      total: 1,
      enabled: 0,
      searchable: 0,
      explore: 0,
    });
    expect(collectionButton.classes()).toContain('unavailable');
    expect(collectionButton.find('strong').text()).toBe('Collection');
    expect(collectionButton.find('span').exists()).toBe(false);
  });
});

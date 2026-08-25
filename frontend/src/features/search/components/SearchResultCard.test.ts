import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it } from 'vitest';
import SearchResultCard from './SearchResultCard.vue';

const i18n = createI18n({ legacy:false,globalInjection:true,locale:'en',messages:{en:{search:{results:{detailsFor:'Preview {name}',preview:'Preview book',sources:'{count} sources'}},app:{common:{unknownAuthor:'Unknown'}}}} });
const result = { name:'Book',author:'Author',coverUrl:'',intro:'',kind:'',lastChapter:'',bookUrl:'/book',sourceUrl:'source',sourceName:'Source',alternateSources:[{sourceUrl:'other',bookUrl:'/other',sourceName:'Other'}] };

describe('SearchResultCard',()=>{
  it('offers both a broad metadata preview target and an explicit preview action',async()=>{
    const wrapper=mount(SearchResultCard,{global:{plugins:[i18n],stubs:{CandidateShelfAction:{template:'<button class="shelf-action">Add</button>'}}},props:{result}});
    expect(wrapper.find('.open-mark').exists()).toBe(false);
    expect(wrapper.findAll('button')).toHaveLength(3);
    expect(wrapper.get('.main').attributes('aria-label')).toBe('Preview Book');
    expect(wrapper.get('.preview-action').text()).toBe('Preview book');
    await wrapper.get('.main').trigger('click');
    await wrapper.get('.preview-action').trigger('click');
    expect(wrapper.emitted('open')).toHaveLength(2);
    expect(wrapper.text()).toContain('2 sources');
    expect(wrapper.find('.shelf-action').exists()).toBe(true);
  });

  it('forwards Search batch recovery state to the candidate action', async () => {
    const wrapper=mount(SearchResultCard,{
      global:{plugins:[i18n],stubs:{CandidateShelfAction:{props:['canContinueSearch','continueSearchCount','searchScanning','searchRetryRequired','searchRestartRequired'],emits:['continue-search','retry-search','restart-search'],template:'<button class="continue" @click="$emit(\'continue-search\')">{{ continueSearchCount }}</button>'}}},
      props:{result,canContinueSearch:true,continueSearchCount:50,searchScanning:false},
    });
    expect(wrapper.get('.continue').text()).toBe('50');
    await wrapper.get('.continue').trigger('click');
    expect(wrapper.emitted('continue-search')).toHaveLength(1);
  });
});

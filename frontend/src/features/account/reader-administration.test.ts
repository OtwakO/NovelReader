import { describe, expect, it } from 'vitest';
import { exactUsernameConfirmed, filterReaderAccounts, readerCounts, replaceReaderAccount } from './reader-administration';
const accounts=[{id:'1',username:'Alice',status:'active' as const,createdAt:1,updatedAt:1},{id:'2',username:'Bob',status:'disabled' as const,createdAt:1,updatedAt:2},{id:'3',username:'Carol',status:'deleting' as const,createdAt:1,updatedAt:3}];
describe('reader administration',()=>{
  it('filters by identity and lifecycle status',()=>{expect(filterReaderAccounts(accounts,'bo','all').map(a=>a.id)).toEqual(['2']);expect(filterReaderAccounts(accounts,'','deleting').map(a=>a.id)).toEqual(['3']);});
  it('summarizes lifecycle states',()=>expect(readerCounts(accounts)).toEqual({total:3,active:1,disabled:1,deleting:1}));
  it('requires exact case-sensitive username confirmation',()=>{expect(exactUsernameConfirmed(accounts[1]!,'Bob')).toBe(true);expect(exactUsernameConfirmed(accounts[1]!,'bob')).toBe(false);});
  it('replaces only the updated account',()=>expect(replaceReaderAccount(accounts,{...accounts[1]!,status:'active'})[1]?.status).toBe('active'));
});

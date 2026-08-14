import type { AdminReaderAccount } from '../../api/auth';

export type ReaderStatusFilter = 'all' | AdminReaderAccount['status'];

export function filterReaderAccounts(accounts: AdminReaderAccount[], query: string, status: ReaderStatusFilter): AdminReaderAccount[] {
  const normalized = query.trim().toLocaleLowerCase();
  return accounts.filter((account) => (status === 'all' || account.status === status)
    && (!normalized || `${account.username}\n${account.id}`.toLocaleLowerCase().includes(normalized)));
}

export function readerCounts(accounts: AdminReaderAccount[]) {
  return {
    total: accounts.length,
    active: accounts.filter((account) => account.status === 'active').length,
    disabled: accounts.filter((account) => account.status === 'disabled').length,
    deleting: accounts.filter((account) => account.status === 'deleting').length,
  };
}

export function exactUsernameConfirmed(account: AdminReaderAccount, confirmation: string): boolean {
  return confirmation === account.username;
}

export function replaceReaderAccount(accounts: AdminReaderAccount[], updated: AdminReaderAccount): AdminReaderAccount[] {
  return accounts.map((account) => account.id === updated.id ? updated : account);
}

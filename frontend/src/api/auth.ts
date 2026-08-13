import { request } from './transport';

export interface SetupStatus { status: 'open' | 'claimed' | 'closed'; available: boolean }
export interface AuthAccount { id: string; username: string; role: 'reader' | 'admin' }
export interface RegistrationPolicy { enabled: boolean; inviteRequired: boolean }
export interface AdminReaderAccount { id: string; username: string; status: 'active' | 'disabled' | 'deleting'; createdAt: number; updatedAt: number }
export interface PasswordResetCredential { token: string; expiresAt: number }
export type RecoveryAction = 'reset_existing' | 'create_replacement';
export interface RecoveryStatus { available: boolean }

export function getSetupStatus() { return request<SetupStatus>('/setup/status'); }
export function createInitialAdministrator(token: string, username: string, password: string) { return request<AuthAccount>('/setup', { method: 'POST', body: JSON.stringify({ token, username, password }) }); }
export function getCurrentAccount() { return request<AuthAccount>('/auth/account'); }
export function getRegistrationPolicy() { return request<RegistrationPolicy>('/auth/registration'); }
export function register(username: string, password: string, inviteCode: string) { return request<AuthAccount>('/auth/register', { method: 'POST', body: JSON.stringify({ username, password, inviteCode }) }); }
export function login(username: string, password: string) { return request<AuthAccount>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }); }
export function changePassword(currentPassword: string, newPassword: string) { return request<void>('/auth/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }) }); }
export async function listReaderAccounts() { return (await request<{ accounts: AdminReaderAccount[] }>('/auth/admin/readers')).accounts; }
export function setReaderEnabled(userID: string, enabled: boolean) { return request<AdminReaderAccount>(`/auth/admin/readers/${encodeURIComponent(userID)}/status`, { method: 'PUT', body: JSON.stringify({ enabled }) }); }
export function issueReaderPasswordReset(userID: string) { return request<PasswordResetCredential>(`/auth/admin/readers/${encodeURIComponent(userID)}/password-reset`, { method: 'POST' }); }
export function deleteReaderAccount(userID: string, username: string) { return request<{ status: 'complete' }>(`/auth/admin/readers/${encodeURIComponent(userID)}`, { method: 'DELETE', body: JSON.stringify({ username }) }); }
export function completePasswordReset(token: string, newPassword: string) { return request<void>('/auth/password-reset', { method: 'POST', body: JSON.stringify({ token, newPassword }) }); }
export function logout() { return request<void>('/auth/logout', { method: 'POST' }); }
export function getRecoveryStatus() { return request<RecoveryStatus>('/recovery/status'); }
export function recoverAdministrator(token: string, action: RecoveryAction, username: string, password: string) { return request<AuthAccount>('/recovery', { method: 'POST', body: JSON.stringify({ token, action, username, password }) }); }

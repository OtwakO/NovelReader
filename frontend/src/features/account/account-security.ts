export interface PasswordFields { currentPassword: string; newPassword: string; confirmPassword: string }
export type PasswordValidation = 'ok' | 'mismatch' | 'tooShort' | 'tooLong' | 'same';

export function validatePasswordChange(fields: PasswordFields): PasswordValidation {
  if (fields.newPassword !== fields.confirmPassword) return 'mismatch';
  if (fields.newPassword.length < 12) return 'tooShort';
  if (fields.newPassword.length > 128) return 'tooLong';
  if (fields.newPassword === fields.currentPassword) return 'same';
  return 'ok';
}

export function clearedPasswordFields(): PasswordFields {
  return { currentPassword: '', newPassword: '', confirmPassword: '' };
}

export function roleLabelKey(role: 'reader' | 'admin'): 'account.profile.readerRole' | 'account.profile.adminRole' {
  return role === 'admin' ? 'account.profile.adminRole' : 'account.profile.readerRole';
}

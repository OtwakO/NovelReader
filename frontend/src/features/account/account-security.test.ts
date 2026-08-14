import { describe, expect, it } from 'vitest';
import { clearedPasswordFields, roleLabelKey, validatePasswordChange } from './account-security';

describe('account security', () => {
  it('validates exact confirmation and backend password bounds', () => { expect(validatePasswordChange({currentPassword:'old password value',newPassword:'replacement password',confirmPassword:'different password'})).toBe('mismatch'); expect(validatePasswordChange({currentPassword:'old password value',newPassword:'short',confirmPassword:'short'})).toBe('tooShort'); expect(validatePasswordChange({currentPassword:'old password value',newPassword:'x'.repeat(129),confirmPassword:'x'.repeat(129)})).toBe('tooLong'); expect(validatePasswordChange({currentPassword:'same password value',newPassword:'same password value',confirmPassword:'same password value'})).toBe('same'); expect(validatePasswordChange({currentPassword:'old password value',newPassword:'replacement password',confirmPassword:'replacement password'})).toBe('ok'); });
  it('clears every credential field after an attempt', () => expect(clearedPasswordFields()).toEqual({currentPassword:'',newPassword:'',confirmPassword:''}));
  it('maps public account roles to localized labels', () => { expect(roleLabelKey('reader')).toBe('account.profile.readerRole'); expect(roleLabelKey('admin')).toBe('account.profile.adminRole'); });
});

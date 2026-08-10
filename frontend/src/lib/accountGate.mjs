export function passwordChangeSucceeded() {
  return {
    gate: 'login',
    account: null,
    hash: '#/login',
    message: 'Password changed. Sign in again with your new password.',
  };
}

export function clearedPasswordFields() {
  return { currentPassword: '', newPassword: '', confirmPassword: '' };
}

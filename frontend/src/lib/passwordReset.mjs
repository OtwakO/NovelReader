export function clearedResetFields() {
  return { token: '', newPassword: '', confirmPassword: '' };
}

export function passwordsMatch(newPassword, confirmPassword) {
  return newPassword === confirmPassword;
}

export function publicResetGate(hash) {
  return hash === '#/password-reset' ? { gate: 'password-reset', account: null } : null;
}

export function resetDelivery(reader, credential) {
  return {
    readerID: reader.id,
    username: reader.username,
    token: credential.token,
    expiresAt: credential.expiresAt,
  };
}

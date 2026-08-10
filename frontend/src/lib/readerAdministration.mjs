export function readerStatusControl(status) {
  switch (status) {
    case 'active':
      return { label: 'Disable', enabled: false, confirmDisable: true, available: true };
    case 'disabled':
      return { label: 'Re-enable', enabled: true, confirmDisable: false, available: true };
    default:
      return { label: 'Unavailable', enabled: false, confirmDisable: false, available: false };
  }
}

export function deletionControl(status) {
  return status === 'deleting'
    ? { label: 'Retry deletion', requiresConfirmation: false }
    : { label: 'Delete account', requiresConfirmation: true };
}

export function deletionConfirmationMatches(username, confirmation) {
  return username === confirmation;
}

export function mayManageReaders(role) {
  return role === 'admin';
}

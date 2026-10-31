export function isStaffOrAdmin(level?: string | number | null): boolean {
  if (!level) return false
  const lvl = String(level).toLowerCase().trim()
  return (
    lvl === 'admin' ||
    lvl === 'moderator' ||
    lvl === 'superadmin' ||
    lvl === 'staff' ||
    lvl === '1' ||
    lvl === '2'
  )
}

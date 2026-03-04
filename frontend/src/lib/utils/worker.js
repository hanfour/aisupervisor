/**
 * Calculate age from a birthday string (YYYY-MM-DD).
 * @param {string} birthday
 * @returns {number|null}
 */
export function calcAge(birthday) {
  if (!birthday) return null
  const bd = new Date(birthday)
  const now = new Date()
  let age = now.getFullYear() - bd.getFullYear()
  const m = now.getMonth() - bd.getMonth()
  if (m < 0 || (m === 0 && now.getDate() < bd.getDate())) age--
  return age
}

/**
 * Return a gender icon character.
 * @param {string} gender - 'male' or 'female'
 * @returns {string}
 */
export function genderIcon(gender) {
  if (gender === 'female') return '\u2640'
  if (gender === 'male') return '\u2642'
  return ''
}

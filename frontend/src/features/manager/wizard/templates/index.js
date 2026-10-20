// Registry of ready-made assessment templates (plain data) and the pure prefill builder.
import constructionStaff from './construction-staff.json'

export { buildTemplateState } from './build.js'
export const TEMPLATES = { [constructionStaff.id]: constructionStaff }

// Single population filter (sent as `departmentId`; each tab keeps its own URL key): all employees, "No department" or one department.
import { useTranslation } from '../../i18n'
import Select from '../../ui/Select'
import { NO_DEPARTMENT } from './analyticsState'

export default function DepartmentFilter({ departments, value, onChange, className = 'w-full sm:w-64' }) {
  const { t } = useTranslation()
  const toValue = (v) => (v === '' ? null : v === NO_DEPARTMENT ? NO_DEPARTMENT : Number(v))
  return (
    <Select
      label={t('analytics.filter.department')}
      value={value === null || value === undefined ? '' : String(value)}
      onChange={(e) => onChange(toValue(e.target.value))}
      className={className}
    >
      <option value="">{t('analytics.filter.all_departments')}</option>
      <option value={NO_DEPARTMENT}>{t('stats.no_department')}</option>
      {(departments || []).map((d) => (
        <option key={d.id} value={String(d.id)}>
          {d.active ? d.name : `${d.name} (${t('stats.department_inactive')})`}
        </option>
      ))}
    </Select>
  )
}

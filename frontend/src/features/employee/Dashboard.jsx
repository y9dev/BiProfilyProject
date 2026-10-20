// Main page. Employees: skills first (widget + radar), then trends, tests to take, history (unchanged).
// Manager/admin (dashboardVariant 'staff'/'staff-gate'): CompanyTrendCard in the radar slot and TeamStatus in
// the trends slot, each fetching on its own; everything else keeps its place.
// On xl the widget is a fixed 20rem column beside the radar; below xl everything stacks.
import { useEffect } from 'react'
import { useAuth } from '../../auth/context'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import PageHeader from '../../ui/PageHeader'
import Skeleton from '../../ui/Skeleton'
import { DashboardTrendCard } from '../analytics/CompanyTrendCard'
import { dashboardVariant } from '../analytics/dashboardVariant'
import TeamStatus from '../analytics/TeamStatus'
import ManagerEntryPoints from '../manager/ManagerEntryPoints'
import AssessmentCatalog from './AssessmentCatalog'
import { purgeOldDrafts } from './draftStore'
import ProfileWidget from './ProfileWidget'
import StatsLocked from './StatsLocked'
import StatsPanel from './StatsPanel'
import Timeline from './Timeline'
import TrendsPanel from './TrendsPanel'

function DashboardSkeleton() {
  return (
    <div className="space-y-6" aria-busy="true">
      <div className="grid gap-6 xl:grid-cols-[20rem_minmax(0,1fr)]">
        <Skeleton className="h-[30rem] w-full" />
        <Skeleton className="h-[30rem] w-full" />
      </div>
      <Skeleton className="h-64 w-full" />
    </div>
  )
}

export default function Dashboard() {
  const { t } = useTranslation()
  const { me } = useAuth()
  const { status, data, reload } = useAsync(
    async (signal) => {
      const [profile, assessments] = await Promise.all([
        endpoints.employee.profile({ signal }),
        endpoints.employee.assessments({ signal }),
      ])
      return { profile, assessments }
    },
    [],
  )

  useEffect(() => {
    purgeOldDrafts()
  }, [])

  return (
    <div className="min-w-0">
      <PageHeader title={t('employee.dashboard.title')} description={t('employee.dashboard.desc')} />
      <div className="space-y-6">
        <ManagerEntryPoints />
        <Async status={status} onRetry={reload} skeleton={<DashboardSkeleton />}>
          {data && (() => {
            const variant = dashboardVariant({ role: me?.role, completed: data.profile.stats.completed })
            if (variant === 'employee-gate' || variant === 'staff-gate') {
              // First-test gate: only the first test (the API returns just that one) and no metrics yet.
              // Staff see the company cards instead of StatsLocked.
              return (
                <>
                  <div className="grid items-stretch gap-6 xl:grid-cols-[20rem_minmax(0,1fr)]">
                    <ProfileWidget me={me} />
                    <AssessmentCatalog
                      items={data.assessments}
                      title={t('employee.first.title')}
                      intro={t('employee.first.intro')}
                    />
                  </div>
                  {variant === 'staff-gate' ? (
                    <>
                      <DashboardTrendCard />
                      <TeamStatus />
                    </>
                  ) : (
                    <StatsLocked />
                  )}
                </>
              )
            }
            const staff = variant === 'staff'
            return (
              <>
                <div className="grid items-stretch gap-6 xl:grid-cols-[20rem_minmax(0,1fr)]">
                  <ProfileWidget me={me} />
                  {staff ? <DashboardTrendCard /> : <StatsPanel profile={data.profile} />}
                </div>
                {staff ? <TeamStatus /> : <TrendsPanel profile={data.profile} />}
                <AssessmentCatalog
                  items={data.assessments.filter((a) => !a.completed)}
                  allDone={data.assessments.length > 0}
                />
                <Timeline timeline={data.profile.timeline} />
              </>
            )
          })()}
        </Async>
      </div>
    </div>
  )
}

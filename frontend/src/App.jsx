// Route table only. Every page lives in features/<area>/... and is lazy-loaded here.
//   public            /login /register
//   signed in         /onboarding/avatar            (profile may be incomplete)
//   signed in + gate  /  /assessments/:id/take  /results/:attemptId
//   manager|admin     /manage/*
//   admin             /admin  /analytics
import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { PublicOnly, RequireAuth, RequireAvatar, RequireRole } from './auth/guards'
import AppShell from './ui/AppShell'
import AuthLayout from './ui/AuthLayout'
import { NotFoundPage } from './ui/StatusPages'

const LoginPage = lazy(() => import('./features/auth/LoginPage'))
const RegisterPage = lazy(() => import('./features/auth/RegisterPage'))
const AvatarOnboardingPage = lazy(() => import('./features/auth/AvatarOnboardingPage'))
const Dashboard = lazy(() => import('./features/employee/Dashboard'))
const TakeAssessmentPage = lazy(() => import('./features/employee/TakeAssessmentPage'))
const ResultPage = lazy(() => import('./features/employee/ResultPage'))
const InvitationsPage = lazy(() => import('./features/manager/invitations/InvitationsPage'))
const AssessmentsPage = lazy(() => import('./features/manager/assessments/AssessmentsPage'))
const AssessmentWizard = lazy(() => import('./features/manager/wizard/AssessmentWizard'))
const CharacteristicsPage = lazy(() => import('./features/manager/characteristics/CharacteristicsPage'))
const DepartmentsPage = lazy(() => import('./features/manager/departments/DepartmentsPage'))
const AdminPage = lazy(() => import('./features/admin/AdminPage'))
const AnalyticsPage = lazy(() => import('./features/analytics/AnalyticsPage'))

const MANAGER_ROLES = ['manager', 'admin']

export default function App() {
  return (
    <Routes>
      <Route element={<PublicOnly />}>
        <Route element={<AuthLayout />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
        </Route>
      </Route>

      <Route element={<RequireAuth />}>
        <Route element={<AppShell minimal />}>
          <Route path="/onboarding/avatar" element={<AvatarOnboardingPage />} />
        </Route>

        <Route element={<RequireAvatar />}>
          <Route element={<AppShell />}>
            <Route index element={<Dashboard />} />
            <Route path="/assessments/:id/take" element={<TakeAssessmentPage />} />
            <Route path="/results/:attemptId" element={<ResultPage />} />

            <Route element={<RequireRole roles={MANAGER_ROLES} />}>
              <Route path="/manage" element={<Navigate to="/manage/invitations" replace />} />
              <Route path="/manage/invitations" element={<InvitationsPage />} />
              <Route path="/manage/assessments" element={<AssessmentsPage />} />
              <Route path="/manage/assessments/new" element={<AssessmentWizard />} />
              <Route path="/manage/characteristics" element={<CharacteristicsPage />} />
              <Route path="/manage/departments" element={<DepartmentsPage />} />
            </Route>

            <Route element={<RequireRole roles={['admin']} />}>
              <Route path="/admin" element={<AdminPage />} />
              <Route path="/analytics" element={<AnalyticsPage />} />
            </Route>

            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  )
}

import { Navigate, Route, Routes } from 'react-router-dom'
import AppLayout from '../App'
import { DashboardPage } from '../pages/Dashboard'
import { LoginPage } from '../pages/Login'
import { BookingsPage } from '../pages/Bookings'
import { ContentPage } from '../pages/Content'
import { TasksPage } from '../pages/Tasks'
import { ReportsPage } from '../pages/Reports'
import { AdminPage } from '../pages/Admin'
import { SchedulePage } from '../pages/Schedule'
import { LearnerPage } from '../pages/Learner'
import { ProtectedRoute } from './ProtectedRoute'

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator', 'instructor', 'learner']} />}>
          <Route path="/dashboard" element={<DashboardPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator', 'learner']} />}>
          <Route path="/bookings" element={<BookingsPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator', 'instructor', 'learner']} />}>
          <Route path="/content" element={<ContentPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator']} />}>
          <Route path="/schedule" element={<SchedulePage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator', 'instructor']} />}>
          <Route path="/tasks" element={<TasksPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin', 'coordinator']} />}>
          <Route path="/reports" element={<ReportsPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['admin']} />}>
          <Route path="/admin" element={<AdminPage />} />
        </Route>
        <Route element={<ProtectedRoute allowedRoles={['learner']} />}>
          <Route path="/learner" element={<LearnerPage />} />
        </Route>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

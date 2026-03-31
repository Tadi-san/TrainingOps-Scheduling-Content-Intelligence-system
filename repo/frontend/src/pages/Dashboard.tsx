import { KPICard } from '../components/KPICard'
import { useAuth } from '../context/AuthContext'
import { useDashboard } from '../hooks/useDashboard'
import { formatTime } from '../services/api'

export function DashboardPage() {
  const { currentUser } = useAuth()
  const role = currentUser?.role ?? 'learner'
  const { dashboard, loading, error } = useDashboard(role, Boolean(currentUser))

  if (loading) return <div className="loading-card glass">Loading dashboard...</div>
  if (error) return <div className="error-banner">{error}</div>
  if (!dashboard) return <div className="loading-card glass">No dashboard data available.</div>

  return (
    <>
      <header className="hero glass">
        <div>
          <p className="eyebrow">Workspace</p>
          <h2>{dashboard.title}</h2>
          <p className="lede">{dashboard.subtitle}</p>
        </div>
      </header>
      <section className="kpi-grid">
        {dashboard.kpis.map((kpi) => (
          <KPICard key={kpi.label} kpi={kpi} />
        ))}
      </section>
      <section className="content-grid">
        <article className="panel glass panel-wide">
          <div className="panel-header">
            <h3>Occupancy heatmap</h3>
            <p>Live occupancy projection by day/hour.</p>
          </div>
          <div className="heatmap">
            {dashboard.heatmap.map((cell) => (
              <div key={`${cell.day}-${cell.hour}`} className={`heat ${cell.state}`} title={`${cell.day} ${cell.hour}:00 - ${cell.load}%`}>
                <span>{cell.day}</span>
                <strong>{cell.hour}:00</strong>
              </div>
            ))}
          </div>
        </article>
        <article className="panel glass panel-wide">
          <div className="panel-header">
            <h3>Session calendar</h3>
            <p>Role-scoped schedule view.</p>
          </div>
          <div className="calendar-grid" role="table" aria-label="Session calendar">
            <div className="calendar-head" role="row">
              <span role="columnheader">Session</span>
              <span role="columnheader">Time</span>
              <span role="columnheader">Room</span>
              <span role="columnheader">Owner</span>
              <span role="columnheader">Status</span>
            </div>
            {dashboard.calendar.map((session) => (
              <div className="calendar-row" key={session.id} role="row">
                <strong role="cell">{session.title}</strong>
                <span role="cell">
                  {formatTime(session.startsAt)} - {formatTime(session.endsAt)}
                </span>
                <span role="cell">{session.room}</span>
                <span role="cell">{session.owner}</span>
                <span className={`status-pill ${session.status}`} role="cell">
                  {session.status}
                </span>
              </div>
            ))}
          </div>
        </article>
      </section>
    </>
  )
}

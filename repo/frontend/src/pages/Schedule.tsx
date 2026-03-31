import { useAuth } from '../context/AuthContext'
import { useScheduleRules } from '../hooks/useScheduleRules'

export function SchedulePage() {
  const { currentUser } = useAuth()
  const role = currentUser?.role ?? 'learner'
  const canEdit = role === 'admin' || role === 'coordinator'
  const {
    periods,
    blackouts,
    periodDraft,
    setPeriodDraft,
    blackoutDraft,
    setBlackoutDraft,
    createPeriod,
    deletePeriod,
    createBlackout,
    deleteBlackout,
    loading,
    message,
    weekdayLabel,
  } = useScheduleRules(canEdit)

  return (
    <section className="content-grid">
      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Class periods</h3>
          <p>Maintain availability windows for room and instructor scheduling.</p>
        </div>
        {canEdit ? (
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr auto', gap: 8, alignItems: 'end' }}>
            <label>
              Period title
              <input value={periodDraft.title} onChange={(e) => setPeriodDraft({ ...periodDraft, title: e.target.value })} />
            </label>
            <label>
              Start
              <input type="time" value={periodDraft.start_time} onChange={(e) => setPeriodDraft({ ...periodDraft, start_time: e.target.value })} />
            </label>
            <label>
              End
              <input type="time" value={periodDraft.end_time} onChange={(e) => setPeriodDraft({ ...periodDraft, end_time: e.target.value })} />
            </label>
            <label>
              Weekday
              <select value={periodDraft.weekday} onChange={(e) => setPeriodDraft({ ...periodDraft, weekday: Number(e.target.value) })}>
                {[0, 1, 2, 3, 4, 5, 6].map((weekday) => (
                  <option key={weekday} value={weekday}>
                    {weekdayLabel(weekday)}
                  </option>
                ))}
              </select>
            </label>
            <button className="btn-secondary" onClick={createPeriod} disabled={loading}>
              Add period
            </button>
          </div>
        ) : null}
        <table className="data-grid">
          <thead>
            <tr>
              <th>Title</th>
              <th>Weekday</th>
              <th>Window</th>
              {canEdit ? <th /> : null}
            </tr>
          </thead>
          <tbody>
            {periods.map((period) => (
              <tr key={period.id}>
                <td>{period.title}</td>
                <td>{weekdayLabel(period.weekday)}</td>
                <td>
                  {period.start_time} - {period.end_time}
                </td>
                {canEdit ? (
                  <td>
                    <button className="btn-secondary" onClick={() => deletePeriod(period.id)} disabled={loading}>
                      Delete
                    </button>
                  </td>
                ) : null}
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Blackout dates</h3>
          <p>Dates that should be blocked from bookings.</p>
        </div>
        {canEdit ? (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr auto', gap: 8, alignItems: 'end' }}>
            <label>
              Blackout date
              <input type="date" value={blackoutDraft.date} onChange={(e) => setBlackoutDraft({ ...blackoutDraft, date: e.target.value })} />
            </label>
            <label>
              Reason
              <input value={blackoutDraft.reason} onChange={(e) => setBlackoutDraft({ ...blackoutDraft, reason: e.target.value })} />
            </label>
            <button className="btn-secondary" onClick={createBlackout} disabled={loading}>
              Add blackout
            </button>
          </div>
        ) : null}
        <table className="data-grid">
          <thead>
            <tr>
              <th>Date</th>
              <th>Reason</th>
              {canEdit ? <th /> : null}
            </tr>
          </thead>
          <tbody>
            {blackouts.map((blackout) => (
              <tr key={blackout.id}>
                <td>{blackout.date}</td>
                <td>{blackout.reason || '-'}</td>
                {canEdit ? (
                  <td>
                    <button className="btn-secondary" onClick={() => deleteBlackout(blackout.id)} disabled={loading}>
                      Delete
                    </button>
                  </td>
                ) : null}
              </tr>
            ))}
          </tbody>
        </table>
        {message ? <p className="workspace-note">{message}</p> : null}
      </article>
    </section>
  )
}

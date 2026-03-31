import { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { useBookings } from '../hooks/useBookings'
import { useScheduleRules } from '../hooks/useScheduleRules'

export function BookingsPage() {
  const { currentUser } = useAuth()
  const {
    form,
    setForm,
    activeBooking,
    rooms,
    instructors,
    conflicts,
    alternatives,
    actionMessage,
    loading,
    holdCountdown,
    holdExpiresSoon,
    cancelDisabled,
    hold,
    confirm,
    cancel,
    reschedule,
    checkIn,
  } = useBookings()
  const [rescheduleStart, setRescheduleStart] = useState('')
  const [rescheduleEnd, setRescheduleEnd] = useState('')
  const role = currentUser?.role ?? 'learner'
  const canCreateHold = role === 'admin' || role === 'coordinator' || role === 'learner'
  const canCheckIn = role === 'instructor' || role === 'coordinator'
  const canReschedule = role === 'admin' || role === 'coordinator'
  const canCancel = role === 'admin' || role === 'coordinator' || role === 'learner'
  const canConfirm = role === 'admin' || role === 'coordinator' || role === 'learner'
  const canManageScheduleRules = role === 'admin' || role === 'coordinator'
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
    message: scheduleMessage,
    loading: scheduleLoading,
    weekdayLabel,
  } = useScheduleRules(canManageScheduleRules)

  return (
    <section className="content-grid">
      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Booking workflow</h3>
          <p>Hold → confirm/cancel/reschedule/check-in with backend state changes.</p>
        </div>
        <div className="auth-form-grid">
          <label>
            Room ID
            <select value={form.room_id} onChange={(e) => setForm({ ...form, room_id: e.target.value })} disabled={!canCreateHold}>
              {rooms.map((room) => (
                <option key={room.ID} value={room.ID}>
                  {room.Name} (cap {room.Capacity})
                </option>
              ))}
            </select>
          </label>
          <label>
            Instructor ID
            <select value={form.instructor_id} onChange={(e) => setForm({ ...form, instructor_id: e.target.value })} disabled={!canCreateHold}>
              {instructors.map((instructor) => (
                <option key={instructor.ID} value={instructor.ID}>
                  {instructor.Name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Title
            <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} disabled={!canCreateHold} />
          </label>
          <label>
            Start
            <input type="datetime-local" value={form.start_at} onChange={(e) => setForm({ ...form, start_at: e.target.value })} disabled={!canCreateHold} />
          </label>
          <label>
            End
            <input type="datetime-local" value={form.end_at} onChange={(e) => setForm({ ...form, end_at: e.target.value })} disabled={!canCreateHold} />
          </label>
          <label>
            Capacity
            <input type="number" min={1} value={form.capacity} onChange={(e) => setForm({ ...form, capacity: Number(e.target.value) })} disabled={!canCreateHold} />
          </label>
          <label>
            Attendees
            <input type="number" min={1} value={form.attendees} onChange={(e) => setForm({ ...form, attendees: Number(e.target.value) })} disabled={!canCreateHold} />
          </label>
          <label>
            Reason
            <input value={form.why} onChange={(e) => setForm({ ...form, why: e.target.value })} />
          </label>
        </div>

        <div className="countdown">
          <strong>{holdCountdown}</strong>
          <span>
            {holdCountdown === '00:00'
              ? 'Hold expired, session may be released.'
              : holdExpiresSoon
                ? 'Warning: less than 1 minute left before auto-release.'
                : 'Auto-release warning: hold expires after 5 minutes'}
          </span>
        </div>

        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="btn-primary" onClick={hold} disabled={loading || !canCreateHold}>
            Start Hold
          </button>
          <button className="btn-secondary" onClick={confirm} disabled={loading || !activeBooking || !canConfirm}>
            Confirm
          </button>
          <button className="btn-secondary" onClick={cancel} disabled={loading || !activeBooking || cancelDisabled || !canCancel}>
            Cancel
          </button>
          <button className="btn-secondary" onClick={checkIn} disabled={loading || !activeBooking || !canCheckIn}>
            Check-in
          </button>
        </div>

        <div style={{ display: 'flex', gap: 8, alignItems: 'end', flexWrap: 'wrap' }}>
          <label>
            Reschedule Start
            <input type="datetime-local" value={rescheduleStart} onChange={(e) => setRescheduleStart(e.target.value)} />
          </label>
          <label>
            Reschedule End
            <input type="datetime-local" value={rescheduleEnd} onChange={(e) => setRescheduleEnd(e.target.value)} />
          </label>
          <button className="btn-secondary" disabled={!activeBooking || loading || !canReschedule} onClick={() => reschedule(rescheduleStart, rescheduleEnd)}>
            Reschedule
          </button>
        </div>

        {actionMessage ? <p className="workspace-note">{actionMessage}</p> : null}
        {cancelDisabled && activeBooking ? <p className="workspace-note">Cancellation disabled within 24-hour cutoff.</p> : null}
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Conflicts</h3>
          <p>Conflict reasons returned by backend.</p>
        </div>
        {conflicts.length === 0 ? <p className="workspace-note">No conflicts</p> : null}
        {conflicts.map((conflict, index) => (
          <p key={`${conflict.Reason}-${index}`} className="workspace-note">
            {conflict.Reason}: {conflict.Detail}
          </p>
        ))}
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Alternatives</h3>
          <p>Suggested fallback slots (up to 3).</p>
        </div>
        {alternatives.length === 0 ? <p className="workspace-note">No alternatives available</p> : null}
        {alternatives.slice(0, 3).map((slot, index) => (
          <button
            key={`${slot.RoomID}-${slot.StartAt}-${index}`}
            className="menu-item"
            onClick={() =>
              setForm({
                ...form,
                room_id: slot.RoomID,
                instructor_id: slot.InstructorID,
                start_at: slot.StartAt.slice(0, 16),
                end_at: slot.EndAt.slice(0, 16),
              })
            }
          >
            {slot.Reason} - {slot.RoomID} / {slot.InstructorID}
          </button>
        ))}
      </article>

      {canManageScheduleRules ? (
        <article className="panel glass panel-wide">
          <div className="panel-header">
            <h3>Class periods and blackout dates</h3>
            <p>Define calendar windows and academic blackout rules.</p>
          </div>

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
            <button className="btn-secondary" onClick={createPeriod} disabled={scheduleLoading}>
              Add period
            </button>
          </div>

          <table className="data-grid">
            <thead>
              <tr>
                <th>Title</th>
                <th>Weekday</th>
                <th>Window</th>
                <th />
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
                  <td>
                    <button className="btn-secondary" onClick={() => deletePeriod(period.id)} disabled={scheduleLoading}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr auto', gap: 8, alignItems: 'end' }}>
            <label>
              Blackout date
              <input type="date" value={blackoutDraft.date} onChange={(e) => setBlackoutDraft({ ...blackoutDraft, date: e.target.value })} />
            </label>
            <label>
              Reason
              <input value={blackoutDraft.reason} onChange={(e) => setBlackoutDraft({ ...blackoutDraft, reason: e.target.value })} />
            </label>
            <button className="btn-secondary" onClick={createBlackout} disabled={scheduleLoading}>
              Add blackout
            </button>
          </div>

          <table className="data-grid">
            <thead>
              <tr>
                <th>Date</th>
                <th>Reason</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {blackouts.map((blackout) => (
                <tr key={blackout.id}>
                  <td>{blackout.date}</td>
                  <td>{blackout.reason || '-'}</td>
                  <td>
                    <button className="btn-secondary" onClick={() => deleteBlackout(blackout.id)} disabled={scheduleLoading}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {scheduleMessage ? <p className="workspace-note">{scheduleMessage}</p> : null}
        </article>
      ) : null}
    </section>
  )
}

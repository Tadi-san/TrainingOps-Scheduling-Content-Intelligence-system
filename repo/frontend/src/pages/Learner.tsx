import { useEffect, useMemo, useState } from 'react'
import { api, Booking, LearnerReservation } from '../services/api'

export function LearnerPage() {
  const [catalog, setCatalog] = useState<Booking[]>([])
  const [reservations, setReservations] = useState<LearnerReservation[]>([])
  const [roomFilter, setRoomFilter] = useState('')
  const [instructorFilter, setInstructorFilter] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [selectedBookingID, setSelectedBookingID] = useState('')
  const [reserveReason, setReserveReason] = useState('')
  const [downloadFileID, setDownloadFileID] = useState('')
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)

  async function refresh() {
    setLoading(true)
    try {
      const [catalogPayload, reservationPayload] = await Promise.all([
        api.listLearnerCatalog({
          room_id: roomFilter || undefined,
          instructor_id: instructorFilter || undefined,
          from: from || undefined,
          to: to || undefined,
        }),
        api.listLearnerReservations(),
      ])
      setCatalog(Array.isArray(catalogPayload.items) ? catalogPayload.items : [])
      setReservations(Array.isArray(reservationPayload.items) ? reservationPayload.items : [])
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to load learner data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [])

  const selectedBooking = useMemo(() => catalog.find((item) => item.ID === selectedBookingID) ?? null, [catalog, selectedBookingID])

  async function reserveSeat() {
    if (!selectedBookingID) {
      setMessage('Select a session to reserve')
      return
    }
    setLoading(true)
    try {
      await api.reserveLearnerSeat(selectedBookingID, reserveReason.trim())
      setMessage('Reservation created')
      await refresh()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to reserve seat')
    } finally {
      setLoading(false)
    }
  }

  function downloadApprovedFile() {
    if (!downloadFileID.trim()) {
      setMessage('Enter an approved file ID')
      return
    }
    window.open(api.downloadLearnerFile(downloadFileID.trim()), '_blank', 'noopener,noreferrer')
  }

  return (
    <section className="content-grid">
      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Learner catalog</h3>
          <p>Browse sessions, reserve a seat, and view your reservation history.</p>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr auto', gap: 8 }}>
          <input placeholder="Room ID" value={roomFilter} onChange={(e) => setRoomFilter(e.target.value)} />
          <input placeholder="Instructor ID" value={instructorFilter} onChange={(e) => setInstructorFilter(e.target.value)} />
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
          <button className="btn-secondary" onClick={refresh} disabled={loading}>
            Filter
          </button>
        </div>
        <table className="data-grid">
          <thead>
            <tr>
              <th />
              <th>Session</th>
              <th>Room</th>
              <th>Instructor</th>
              <th>Time</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {catalog.map((session) => (
              <tr key={session.ID}>
                <td>
                  <input type="radio" checked={selectedBookingID === session.ID} onChange={() => setSelectedBookingID(session.ID)} />
                </td>
                <td>{session.Title}</td>
                <td>{session.RoomID}</td>
                <td>{session.InstructorID}</td>
                <td>
                  {new Date(session.StartAt).toLocaleString()} - {new Date(session.EndAt).toLocaleTimeString()}
                </td>
                <td>{session.Status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Reserve seat</h3>
          <p>Reserve the selected session with a backend conflict check.</p>
        </div>
        {selectedBooking ? (
          <p className="workspace-note">
            Selected: {selectedBooking.Title} in room {selectedBooking.RoomID}
          </p>
        ) : (
          <p className="workspace-note">Select a session from the catalog.</p>
        )}
        <label>
          Reason
          <input value={reserveReason} onChange={(e) => setReserveReason(e.target.value)} />
        </label>
        <button className="btn-primary" onClick={reserveSeat} disabled={loading || !selectedBookingID}>
          Reserve
        </button>
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>My reservations</h3>
          <p>Live reservation list from the backend.</p>
        </div>
        {reservations.length === 0 ? <p className="workspace-note">No reservations yet</p> : null}
        {reservations.map((reservation) => (
          <div key={reservation.ID} className="dag-card">
            <strong>{reservation.BookingID}</strong>
            <span>{reservation.Status}</span>
            <small>{new Date(reservation.CreatedAt).toLocaleString()}</small>
          </div>
        ))}
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Approved download</h3>
          <p>Download a file by approved file ID with server watermarking.</p>
        </div>
        <label>
          File ID
          <input value={downloadFileID} onChange={(e) => setDownloadFileID(e.target.value)} />
        </label>
        <button className="btn-secondary" onClick={downloadApprovedFile}>
          Open download
        </button>
      </article>

      {message ? (
        <article className="panel glass panel-wide">
          <p className="workspace-note">{message}</p>
        </article>
      ) : null}
    </section>
  )
}

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useBookings } from './useBookings'

function BookingHarness() {
  const { form, setForm, hold, holdCountdown, conflicts } = useBookings()
  return (
    <div>
      <input aria-label="room" value={form.room_id} onChange={(e) => setForm({ ...form, room_id: e.target.value })} />
      <button onClick={() => void hold()}>hold</button>
      <div data-testid="countdown">{holdCountdown}</div>
      {conflicts.map((c, idx) => (
        <div key={`${c.Reason}-${idx}`}>{c.Detail}</div>
      ))}
    </div>
  )
}

describe('useBookings', () => {
  it('calls hold API and renders conflict reasons', async () => {
    const fetchSpy = vi.fn().mockImplementation(async (url: string) => {
      if (url.includes('/v1/bookings/rooms')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ items: [{ ID: 'room-a', Name: 'Room A', Capacity: 20 }] }),
        }
      }
      if (url.includes('/v1/bookings/instructors')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ items: [{ ID: 'inst-1', Name: 'Instructor A' }] }),
        }
      }
      return {
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          conflicts: [{ Reason: 'room', Detail: 'Room occupied' }],
          alternatives: [{ RoomID: 'room-b', InstructorID: 'inst-2', StartAt: '2026-04-01T10:00:00Z', EndAt: '2026-04-01T11:00:00Z', Reason: 'Available' }],
        }),
      }
    })
    vi.stubGlobal('fetch', fetchSpy)

    render(<BookingHarness />)

    fireEvent.change(screen.getByLabelText('room'), { target: { value: 'room-a' } })
    fireEvent.click(screen.getByText('hold'))

    await waitFor(() => expect(screen.getByText('Room occupied')).toBeInTheDocument())
    expect(fetchSpy).toHaveBeenCalledWith(
      expect.stringContaining('/v1/bookings/hold'),
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('shows active countdown when hold is created', async () => {
    vi.useFakeTimers()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (url: string) => {
        if (url.includes('/v1/bookings/rooms')) {
          return {
            ok: true,
            headers: new Headers({ 'content-type': 'application/json' }),
            json: async () => ({ items: [{ ID: 'room-a', Name: 'Room A', Capacity: 20 }] }),
          }
        }
        if (url.includes('/v1/bookings/instructors')) {
          return {
            ok: true,
            headers: new Headers({ 'content-type': 'application/json' }),
            json: async () => ({ items: [{ ID: 'inst-1', Name: 'Instructor A' }] }),
          }
        }
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({
            booking: {
              ID: 'b-1',
              TenantID: 'tenant-1',
              UserID: 'u-1',
              RoomID: 'room-a',
              InstructorID: 'inst-1',
              Title: 'Test',
              StartAt: '2026-04-01T10:00:00Z',
              EndAt: '2026-04-01T11:00:00Z',
              Capacity: 10,
              Attendees: 2,
              Status: 'held',
              HoldExpiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
              RescheduleCount: 0,
            },
          }),
        }
      }),
    )

    render(<BookingHarness />)
    fireEvent.click(screen.getByText('hold'))
    await waitFor(() => expect(screen.getByTestId('countdown').textContent).not.toBe('No hold timer'))
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByTestId('countdown').textContent).toMatch(/\d{2}:\d{2}/)
    vi.useRealTimers()
  })
})

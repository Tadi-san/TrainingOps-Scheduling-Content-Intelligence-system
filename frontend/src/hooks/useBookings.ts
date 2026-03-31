import { useEffect, useMemo, useState } from 'react'
import { AlternativeSlot, api, Booking, BookingConflict, Instructor, Room } from '../services/api'

type BookingForm = {
  room_id: string
  instructor_id: string
  title: string
  start_at: string
  end_at: string
  capacity: number
  attendees: number
  why: string
}

export function useBookings() {
  const [form, setForm] = useState<BookingForm>({
    room_id: '',
    instructor_id: '',
    title: '',
    start_at: '',
    end_at: '',
    capacity: 20,
    attendees: 10,
    why: '',
  })
  const [activeBooking, setActiveBooking] = useState<Booking | null>(null)
  const [conflicts, setConflicts] = useState<BookingConflict[]>([])
  const [alternatives, setAlternatives] = useState<AlternativeSlot[]>([])
  const [actionMessage, setActionMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const [rooms, setRooms] = useState<Room[]>([])
  const [instructors, setInstructors] = useState<Instructor[]>([])
  const [nowTick, setNowTick] = useState(() => Date.now())

  useEffect(() => {
    if (!activeBooking?.HoldExpiresAt) return
    const timer = window.setInterval(() => setNowTick(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [activeBooking?.HoldExpiresAt])

  useEffect(() => {
    let cancelled = false
    Promise.all([api.listRooms(), api.listInstructors()])
      .then(([roomsResult, instructorsResult]) => {
        if (cancelled) return
        const nextRooms = Array.isArray(roomsResult.items) ? roomsResult.items : []
        const nextInstructors = Array.isArray(instructorsResult.items) ? instructorsResult.items : []
        setRooms(nextRooms)
        setInstructors(nextInstructors)
        setForm((current) => ({
          ...current,
          room_id: current.room_id || nextRooms[0]?.ID || '',
          instructor_id: current.instructor_id || nextInstructors[0]?.ID || '',
        }))
      })
      .catch(() => {
        if (cancelled) return
        setActionMessage('Unable to load room/instructor options')
      })
    return () => {
      cancelled = true
    }
  }, [])

  const holdCountdown = useMemo(() => {
    if (!activeBooking?.HoldExpiresAt) return 'No hold timer'
    const remaining = Math.max(0, new Date(activeBooking.HoldExpiresAt).getTime() - nowTick)
    const minutes = Math.floor(remaining / 60000)
    const seconds = Math.floor((remaining % 60000) / 1000)
    return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }, [activeBooking?.HoldExpiresAt, nowTick])

  const holdExpiresSoon = useMemo(() => {
    if (!activeBooking?.HoldExpiresAt) return false
    return new Date(activeBooking.HoldExpiresAt).getTime() - nowTick <= 60_000
  }, [activeBooking?.HoldExpiresAt, nowTick])

  const cancelDisabled = useMemo(() => {
    if (!activeBooking?.StartAt) return true
    const ms = new Date(activeBooking.StartAt).getTime() - Date.now()
    return ms < 24 * 60 * 60 * 1000
  }, [activeBooking?.StartAt])

  async function hold() {
    setLoading(true)
    setActionMessage('')
    setConflicts([])
    setAlternatives([])
    try {
      const response = await api.holdBooking(form)
      if (response.booking) {
        setActiveBooking(response.booking)
        setActionMessage('Booking hold created.')
      } else {
        setConflicts(response.conflicts ?? [])
        setAlternatives(response.alternatives ?? [])
        setActionMessage(response.error ?? 'Booking hold failed.')
      }
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : 'Booking hold failed')
    } finally {
      setLoading(false)
    }
  }

  async function confirm() {
    if (!activeBooking) return
    setLoading(true)
    try {
      await api.confirmBooking(activeBooking.ID, form.why)
      setActiveBooking({ ...activeBooking, Status: 'confirmed', HoldExpiresAt: undefined })
      setActionMessage('Booking confirmed.')
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : 'Confirm failed')
    } finally {
      setLoading(false)
    }
  }

  async function cancel() {
    if (!activeBooking) return
    setLoading(true)
    try {
      await api.cancelBooking(activeBooking.ID, form.why)
      setActiveBooking({ ...activeBooking, Status: 'cancelled' })
      setActionMessage('Booking cancelled.')
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : 'Cancel failed')
    } finally {
      setLoading(false)
    }
  }

  async function reschedule(newStart: string, newEnd: string) {
    if (!activeBooking) return
    setLoading(true)
    try {
      const response = await api.rescheduleBooking(activeBooking.ID, newStart, newEnd, form.why)
      setActiveBooking(response.booking)
      setActionMessage('Booking rescheduled.')
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : 'Reschedule failed')
    } finally {
      setLoading(false)
    }
  }

  async function checkIn() {
    if (!activeBooking) return
    setLoading(true)
    try {
      await api.checkInBooking(activeBooking.ID)
      setActiveBooking({ ...activeBooking, Status: 'checked_in' })
      setActionMessage('Booking checked in.')
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : 'Check-in failed')
    } finally {
      setLoading(false)
    }
  }

  return {
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
  }
}

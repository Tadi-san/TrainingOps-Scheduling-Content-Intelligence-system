import { useEffect, useState } from 'react'
import { api, BlackoutDate, ClassPeriod } from '../services/api'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

export function useScheduleRules(enabled: boolean) {
  const [periods, setPeriods] = useState<ClassPeriod[]>([])
  const [blackouts, setBlackouts] = useState<BlackoutDate[]>([])
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')

  const [periodDraft, setPeriodDraft] = useState({
    title: '',
    start_time: '08:30',
    end_time: '10:00',
    weekday: 1,
  })
  const [blackoutDraft, setBlackoutDraft] = useState({
    date: '',
    reason: '',
  })

  async function refresh() {
    if (!enabled) return
    setLoading(true)
    try {
      const [periodPayload, blackoutPayload] = await Promise.all([api.listClassPeriods(), api.listBlackoutDates()])
      setPeriods(Array.isArray(periodPayload.items) ? periodPayload.items : [])
      setBlackouts(Array.isArray(blackoutPayload.items) ? blackoutPayload.items : [])
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to load schedule rules')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [enabled])

  async function createPeriod() {
    if (!enabled) return
    if (!periodDraft.title.trim()) {
      setMessage('Class period title is required')
      return
    }
    setLoading(true)
    try {
      await api.createClassPeriod({
        title: periodDraft.title.trim(),
        start_time: periodDraft.start_time,
        end_time: periodDraft.end_time,
        weekday: periodDraft.weekday,
      })
      setPeriodDraft({ title: '', start_time: '08:30', end_time: '10:00', weekday: 1 })
      setMessage('Class period created')
      await refresh()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to create class period')
      setLoading(false)
    }
  }

  async function deletePeriod(id: string) {
    if (!enabled) return
    setLoading(true)
    try {
      await api.deleteClassPeriod(id)
      setMessage('Class period removed')
      await refresh()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to remove class period')
      setLoading(false)
    }
  }

  async function createBlackout() {
    if (!enabled) return
    if (!blackoutDraft.date) {
      setMessage('Blackout date is required')
      return
    }
    setLoading(true)
    try {
      await api.createBlackoutDate({ date: blackoutDraft.date, reason: blackoutDraft.reason.trim() })
      setBlackoutDraft({ date: '', reason: '' })
      setMessage('Blackout date created')
      await refresh()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to create blackout date')
      setLoading(false)
    }
  }

  async function deleteBlackout(id: string) {
    if (!enabled) return
    setLoading(true)
    try {
      await api.deleteBlackoutDate(id)
      setMessage('Blackout date removed')
      await refresh()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to remove blackout date')
      setLoading(false)
    }
  }

  return {
    periods,
    blackouts,
    loading,
    message,
    periodDraft,
    setPeriodDraft,
    blackoutDraft,
    setBlackoutDraft,
    createPeriod,
    deletePeriod,
    createBlackout,
    deleteBlackout,
    refresh,
    weekdayLabel: (weekday: number) => WEEKDAYS[weekday] ?? String(weekday),
  }
}

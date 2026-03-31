import { useEffect, useMemo, useState } from 'react'
import { api, Task } from '../services/api'

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const [milestoneID, setMilestoneID] = useState<string>(import.meta.env.VITE_DEFAULT_MILESTONE_ID ?? '')

  const graphLines = useMemo(
    () =>
      tasks.flatMap((task) =>
        task.DependencyIDs.map((dep) => ({
          from: task.Title,
          to: tasks.find((candidate) => candidate.ID === dep)?.Title ?? dep,
        })),
      ),
    [tasks],
  )

  async function refresh() {
    if (!milestoneID.trim()) {
      setTasks([])
      setMessage('Enter a milestone ID to load tasks')
      return
    }
    setLoading(true)
    try {
      const result = await api.listTasks(milestoneID.trim())
      setTasks(result.tasks)
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to load tasks')
    } finally {
      setLoading(false)
    }
  }

  async function saveDependencies(taskID: string, dependencyIDs: string[]) {
    const all = tasks.map((task) => (task.ID === taskID ? { ...task, DependencyIDs: dependencyIDs } : task))
    if (hasCycle(all)) {
      setMessage('Circular dependency detected')
      return
    }
    try {
      await api.addTaskDependencies(taskID, dependencyIDs)
      await refresh()
      setMessage('Dependencies updated')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Dependency update failed')
    }
  }

  async function updateTask(task: Task) {
    try {
      await api.updateTask(task.ID, {
        title: task.Title,
        description: task.Description,
        due_date: task.DueDate ?? '',
        dependency_ids: task.DependencyIDs,
        estimated_minutes: task.EstimatedMinutes,
        actual_minutes: task.ActualMinutes,
        expected_version: task.Version,
      })
      await refresh()
      setMessage('Task updated')
    } catch (error) {
      const text = error instanceof Error ? error.message : 'Task update failed'
      if (text.toLowerCase().includes('updated by another user')) {
        setMessage('Stale version detected. Refreshing tasks.')
        await refresh()
      } else {
        setMessage(text)
      }
    }
  }

  async function bulkMarkComplete() {
    for (const id of selected) {
      const task = tasks.find((item) => item.ID === id)
      if (!task) continue
      await updateTask({ ...task, ActualMinutes: Math.max(task.ActualMinutes, task.EstimatedMinutes) })
    }
    setSelected([])
  }

  useEffect(() => {
    refresh()
  }, [milestoneID])

  return {
    tasks,
    selected,
    setSelected,
    milestoneID,
    setMilestoneID,
    graphLines,
    message,
    loading,
    refresh,
    saveDependencies,
    updateTask,
    bulkMarkComplete,
  }
}

function hasCycle(tasks: Task[]) {
  const graph = new Map<string, string[]>()
  tasks.forEach((task) => graph.set(task.ID, [...(task.DependencyIDs ?? [])]))
  const visited = new Set<string>()
  const stack = new Set<string>()
  const visit = (id: string): boolean => {
    if (stack.has(id)) return true
    if (visited.has(id)) return false
    visited.add(id)
    stack.add(id)
    for (const dep of graph.get(id) ?? []) if (visit(dep)) return true
    stack.delete(id)
    return false
  }
  for (const id of graph.keys()) if (visit(id)) return true
  return false
}

import { useTasks } from '../hooks/useTasks'
import { useAuth } from '../context/AuthContext'
import { useState } from 'react'

export function TasksPage() {
  const { currentUser } = useAuth()
  const role = currentUser?.role ?? 'learner'
  const canEdit = role === 'admin' || role === 'coordinator'
  const { tasks, selected, setSelected, milestoneID, setMilestoneID, graphLines, message, loading, refresh, saveDependencies, bulkMarkComplete, updateTask } = useTasks()
  const [dueDateDrafts, setDueDateDrafts] = useState<Record<string, string>>({})

  return (
    <section className="content-grid">
      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Task dependencies</h3>
          <p>DAG preflight + optimistic locking retry.</p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <input
            style={{ minWidth: 280 }}
            placeholder="Milestone ID"
            value={milestoneID}
            onChange={(e) => setMilestoneID(e.target.value)}
          />
          <button className="btn-secondary" onClick={refresh} disabled={loading}>
            Refresh
          </button>
          <button className="btn-primary" onClick={bulkMarkComplete} disabled={selected.length === 0 || !canEdit}>
            Mark selected complete
          </button>
        </div>
        {message ? <p className="workspace-note">{message}</p> : null}
        <table className="data-grid">
          <thead>
            <tr>
              <th />
              <th>Task</th>
              <th>Due Date</th>
              <th>Depends On</th>
              <th>Version</th>
              <th>Effort</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((task) => (
              <tr key={task.ID}>
                <td>
                  <input
                    type="checkbox"
                    checked={selected.includes(task.ID)}
                    disabled={!canEdit}
                    onChange={(e) =>
                      setSelected((current) => (e.target.checked ? [...current, task.ID] : current.filter((id) => id !== task.ID)))
                    }
                  />
                </td>
                <td>{task.Title}</td>
                <td>
                  {canEdit ? (
                    <div style={{ display: 'grid', gap: 6 }}>
                      <input
                        type="date"
                        value={dueDateDrafts[task.ID] ?? task.DueDate?.slice(0, 10) ?? ''}
                        onChange={(e) => setDueDateDrafts((current) => ({ ...current, [task.ID]: e.target.value }))}
                      />
                      <button
                        className="btn-secondary"
                        onClick={() =>
                          void updateTask({
                            ...task,
                            DueDate: dueDateDrafts[task.ID] ?? task.DueDate ?? '',
                          })
                        }
                      >
                        Save date
                      </button>
                    </div>
                  ) : (
                    task.DueDate?.slice(0, 10) ?? '-'
                  )}
                </td>
                <td>
                  <select
                    value=""
                    disabled={!canEdit}
                    onChange={(e) => {
                      const dep = e.target.value
                      if (!dep) return
                      const next = Array.from(new Set([...(task.DependencyIDs ?? []), dep]))
                      saveDependencies(task.ID, next)
                    }}
                  >
                    <option value="">Add dependency</option>
                    {tasks.filter((candidate) => candidate.ID !== task.ID).map((candidate) => (
                      <option key={candidate.ID} value={candidate.ID}>
                        {candidate.Title}
                      </option>
                    ))}
                  </select>
                </td>
                <td>{task.Version}</td>
                <td>
                  {task.ActualMinutes}/{task.EstimatedMinutes}m
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Dependency graph</h3>
          <p>Simple relationship list.</p>
        </div>
        {graphLines.length === 0 ? <p className="workspace-note">No dependency edges</p> : null}
        {graphLines.map((line, index) => (
          <p key={`${line.from}-${line.to}-${index}`} className="workspace-note">
            {line.from} depends on {line.to}
          </p>
        ))}
      </article>
    </section>
  )
}

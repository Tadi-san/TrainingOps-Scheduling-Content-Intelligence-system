import { KPI } from '../services/api'

export function KPICard({ kpi }: { kpi: KPI }) {
  return (
    <article className="kpi glass">
      <span>{kpi.label}</span>
      <strong>{kpi.value}</strong>
      <small>{kpi.delta}</small>
    </article>
  )
}

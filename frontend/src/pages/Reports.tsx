import { useState } from 'react'
import { api } from '../services/api'

type GeneratedReport = { filename: string; size: number; created_at: string; type: string }

export function ReportsPage() {
  const [reportType, setReportType] = useState<'seat' | 'near-expiry' | 'incident' | 'custom'>('seat')
  const [format, setFormat] = useState<'csv' | 'pdf'>('csv')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [progress, setProgress] = useState(false)
  const [reports, setReports] = useState<GeneratedReport[]>([])
  const [message, setMessage] = useState('')

  async function generate() {
    setProgress(true)
    setMessage('')
    try {
      const payload = await api.generateReport(reportType, format, dateFrom, dateTo)
      setReports((current) => [
        { filename: payload.filename, size: payload.size, created_at: payload.created_at, type: `${reportType}:${format}` },
        ...current,
      ])
      setMessage('Report generated successfully')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Report generation failed')
    } finally {
      setProgress(false)
    }
  }

  return (
    <section className="content-grid">
      <article className="panel glass">
        <div className="panel-header">
          <h3>Generate report</h3>
          <p>CSV/PDF export with download.</p>
        </div>
        <label>
          Report type
          <select value={reportType} onChange={(e) => setReportType(e.target.value as 'seat' | 'near-expiry' | 'incident' | 'custom')}>
            <option value="seat">Seat utilization</option>
            <option value="near-expiry">Near-expiry</option>
            <option value="incident">Incident rates</option>
            <option value="custom">Custom</option>
          </select>
        </label>
        <label>
          Format
          <select value={format} onChange={(e) => setFormat(e.target.value as 'csv' | 'pdf')}>
            <option value="csv">CSV</option>
            <option value="pdf">PDF</option>
          </select>
        </label>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
          <label>
            Date from
            <input type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
          </label>
          <label>
            Date to
            <input type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
          </label>
        </div>
        <button className="btn-primary" onClick={generate} disabled={progress}>
          {progress ? 'Generating...' : 'Generate Report'}
        </button>
        {message ? <p className="workspace-note">{message}</p> : null}
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Generated reports</h3>
          <p>Download previously generated exports.</p>
        </div>
        <table className="data-grid">
          <thead>
            <tr>
              <th>File</th>
              <th>Type</th>
              <th>Created</th>
              <th>Size</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {reports.map((report) => (
              <tr key={report.filename}>
                <td>{report.filename}</td>
                <td>{report.type}</td>
                <td>{new Date(report.created_at).toLocaleString()}</td>
                <td>{report.size}</td>
                <td>
                  <a className="btn-secondary" href={api.reportDownloadURL(report.filename)}>
                    Download
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>
    </section>
  )
}

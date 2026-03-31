import { useEffect, useMemo } from 'react'
import { useAuth } from '../context/AuthContext'
import { useContent } from '../hooks/useContent'

export function ContentPage() {
  const { currentUser } = useAuth()
  const role = currentUser?.role ?? 'learner'
  const canEdit = role === 'admin' || role === 'coordinator'
  const canInspectVersions = role === 'admin' || role === 'coordinator' || role === 'instructor'
  const {
    query,
    setQuery,
    categoryFilter,
    setCategoryFilter,
    tagFilter,
    setTagFilter,
    sortBy,
    setSortBy,
    items,
    selectedItem,
    setSelectedItem,
    versions,
    uploadFile,
    setUploadFile,
    uploadProgress,
    uploadStatus,
    metadata,
    setMetadata,
    duplicateModalOpen,
    duplicateAction,
    setDuplicateAction,
    shareLink,
    uploadInChunks,
    resolveDuplicate,
    generateShareLink,
  } = useContent()

  const previewURL = useMemo(() => {
    if (!uploadFile) return ''
    return URL.createObjectURL(uploadFile)
  }, [uploadFile])

  useEffect(() => {
    return () => {
      if (previewURL) URL.revokeObjectURL(previewURL)
    }
  }, [previewURL])

  return (
    <section className="content-grid">
      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Content catalog</h3>
          <p>Search + category/tag filters + relevance/date sorting.</p>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr', gap: 8 }}>
          <input placeholder="Search content..." value={query} onChange={(e) => setQuery(e.target.value)} />
          <input placeholder="Filter category" value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)} />
          <input placeholder="Filter tag" value={tagFilter} onChange={(e) => setTagFilter(e.target.value)} />
          <select value={sortBy} onChange={(e) => setSortBy(e.target.value as 'relevance' | 'date')}>
            <option value="relevance">Relevance</option>
            <option value="date">Date</option>
          </select>
        </div>
        <table className="data-grid">
          <thead>
            <tr>
              <th>Title</th>
              <th>Category</th>
              <th>Difficulty</th>
              <th>Duration</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.ID} onClick={() => setSelectedItem(item)}>
                <td>{highlight(item.Title, query)}</td>
                <td>{item.CategoryID || '-'}</td>
                <td>{item.Difficulty}</td>
                <td>{item.DurationMinutes}m</td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Upload + metadata</h3>
          <p>Difficulty 1-5 and duration 5-480 validation.</p>
        </div>
        {canEdit ? <input type="file" onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)} disabled={!canEdit} /> : <p className="workspace-note">View-only mode for your role.</p>}
        <label>
          Difficulty
          <select value={metadata.difficulty} onChange={(e) => setMetadata({ ...metadata, difficulty: Number(e.target.value) })} disabled={!canEdit}>
            {[1, 2, 3, 4, 5].map((d) => (
              <option key={d} value={d}>
                {d}
              </option>
            ))}
          </select>
        </label>
        <label>
          Duration (minutes)
          <input type="number" min={5} max={480} value={metadata.durationMinutes} onChange={(e) => setMetadata({ ...metadata, durationMinutes: Number(e.target.value) })} disabled={!canEdit} />
        </label>
        <label>
          Category
          <input value={metadata.category} onChange={(e) => setMetadata({ ...metadata, category: e.target.value })} disabled={!canEdit} />
        </label>
        <label>
          Tags (comma separated)
          <input
            value={metadata.tags.join(',')}
            onChange={(e) =>
              setMetadata({
                ...metadata,
                tags: e.target.value
                  .split(',')
                  .map((t) => t.trim())
                  .filter(Boolean),
              })
            }
            disabled={!canEdit}
          />
        </label>
        {canEdit ? (
          <>
            <button className="btn-primary" onClick={uploadInChunks} disabled={!uploadFile || !canEdit}>
              Upload in chunks
            </button>
            <p className="workspace-note">Progress: {uploadProgress}%</p>
            {uploadStatus ? <p className="workspace-note">{uploadStatus}</p> : null}
          </>
        ) : null}
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Version history</h3>
          <p>Document versions with timestamp and checksum.</p>
        </div>
        {!canInspectVersions ? <p className="workspace-note">Version history is not available for your role.</p> : null}
        {canInspectVersions && versions.length === 0 ? <p className="workspace-note">No versions yet</p> : null}
        {canInspectVersions
          ? versions.map((version) => (
              <div key={version.ID} className="dag-card">
                <strong>{version.FileName}</strong>
                <span>v{version.Version}</span>
                <span>{new Date(version.CreatedAt).toLocaleString()}</span>
                <small>{version.Checksum}</small>
              </div>
            ))
          : null}
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Share link</h3>
          <p>Generate copyable link with max 72h expiry.</p>
        </div>
        <label>
          Expiry (hours)
          <input
            type="number"
            min={1}
            max={72}
            value={metadata.shareExpiryHours}
            onChange={(e) => setMetadata({ ...metadata, shareExpiryHours: Number(e.target.value) })}
            disabled={!selectedItem || !canEdit}
          />
        </label>
        <button className="btn-secondary" onClick={generateShareLink} disabled={!selectedItem || !canEdit}>
          Generate link
        </button>
        {shareLink ? (
          <div className="dag-card">
            <span>{shareLink}</span>
            <button className="btn-secondary" onClick={() => navigator.clipboard.writeText(shareLink)}>
              Copy
            </button>
          </div>
        ) : null}
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Preview</h3>
          <p>Preview sourced from selected uploaded file.</p>
        </div>
        <div className="preview-frame">{previewURL ? <iframe title="uploaded-preview" src={previewURL} /> : <p className="workspace-note">Upload/select a file to preview.</p>}</div>
      </article>

      {duplicateModalOpen ? (
        <div className="modal">
          <div className="modal-header">Duplicate detected</div>
          <div className="modal-body">
            <p>Select merge behavior:</p>
            <label>
              <input type="radio" checked={duplicateAction === 'keep'} onChange={() => setDuplicateAction('keep')} />
              Keep both
            </label>
            <label>
              <input type="radio" checked={duplicateAction === 'replace'} onChange={() => setDuplicateAction('replace')} />
              Replace
            </label>
            <label>
              <input type="radio" checked={duplicateAction === 'merge'} onChange={() => setDuplicateAction('merge')} />
              Merge metadata
            </label>
          </div>
          <div className="modal-footer">
            <button className="btn-primary" onClick={resolveDuplicate}>
              Confirm
            </button>
          </div>
        </div>
      ) : null}
    </section>
  )
}

function highlight(value: string, query: string) {
  if (!query.trim()) return value
  const index = value.toLowerCase().indexOf(query.trim().toLowerCase())
  if (index < 0) return value
  const start = value.slice(0, index)
  const match = value.slice(index, index + query.length)
  const end = value.slice(index + query.length)
  return (
    <span>
      {start}
      <mark>{match}</mark>
      {end}
    </span>
  )
}

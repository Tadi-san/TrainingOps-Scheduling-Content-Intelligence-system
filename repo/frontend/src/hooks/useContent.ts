import { useEffect, useMemo, useState } from 'react'
import { api, ContentItem, DocumentVersion, sha256Hex, uint8ToBase64 } from '../services/api'

type MetadataForm = {
  difficulty: number
  durationMinutes: number
  category: string
  tags: string[]
  shareExpiryHours: number
}

export function useContent() {
  const [query, setQuery] = useState('')
  const [categoryFilter, setCategoryFilter] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [sortBy, setSortBy] = useState<'relevance' | 'date'>('relevance')
  const [items, setItems] = useState<ContentItem[]>([])
  const [selectedItem, setSelectedItem] = useState<ContentItem | null>(null)
  const [versions, setVersions] = useState<DocumentVersion[]>([])
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadStatus, setUploadStatus] = useState('')
  const [metadata, setMetadata] = useState<MetadataForm>({
    difficulty: 1,
    durationMinutes: 30,
    category: '',
    tags: [],
    shareExpiryHours: 72,
  })
  const [duplicateModalOpen, setDuplicateModalOpen] = useState(false)
  const [duplicateAction, setDuplicateAction] = useState<'keep' | 'replace' | 'merge'>('keep')
  const [shareLink, setShareLink] = useState('')
  const [pendingDuplicateChecksum, setPendingDuplicateChecksum] = useState<string | null>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const q = query.trim()
      if (!q) return
      api
        .searchContent(q)
        .then((result) => {
          setItems(result.items)
        })
        .catch(() => setItems([]))
    }, 350)
    return () => window.clearTimeout(timer)
  }, [query])

  useEffect(() => {
    if (!selectedItem) return
    let cancelled = false
    api
      .listDocumentVersions(selectedItem.ID)
      .then((result) => {
        if (!cancelled) setVersions(result.versions)
      })
      .catch(() => {
        if (!cancelled) setVersions([])
      })
    return () => {
      cancelled = true
    }
  }, [selectedItem?.ID])

  const filteredItems = useMemo(() => {
    const byCategory = categoryFilter.trim().toLowerCase()
    const byTag = tagFilter.trim().toLowerCase()
    let next = [...items]
    if (byCategory) next = next.filter((item) => item.CategoryID?.toLowerCase().includes(byCategory))
    if (byTag) next = next.filter((item) => (item.Checksum ?? '').toLowerCase().includes(byTag))
    if (sortBy === 'date') next.sort((a, b) => String(b.UpdatedAt ?? '').localeCompare(String(a.UpdatedAt ?? '')))
    return next
  }, [items, categoryFilter, tagFilter, sortBy])

  function validateMetadata() {
    if (metadata.difficulty < 1 || metadata.difficulty > 5) throw new Error('Difficulty must be 1-5')
    if (metadata.durationMinutes < 5 || metadata.durationMinutes > 480) throw new Error('Duration must be 5-480 minutes')
    if (metadata.shareExpiryHours < 1 || metadata.shareExpiryHours > 72) throw new Error('Share expiry cannot exceed 72 hours')
  }

  async function uploadInChunks() {
    if (!uploadFile) return
    validateMetadata()
    setUploadStatus('')
    setUploadProgress(0)

    const bytes = new Uint8Array(await uploadFile.arrayBuffer())
    const checksum = await sha256Hex(bytes)
    const duplicate = items.find((item) => item.Checksum && item.Checksum === checksum)
    if (duplicate) {
      setSelectedItem(duplicate)
      setPendingDuplicateChecksum(checksum)
      setDuplicateModalOpen(true)
      return
    }
    await continueUpload(checksum)
  }

  async function continueUpload(precomputedChecksum?: string) {
    if (!uploadFile) return
    const chunkSize = 256 * 1024
    const totalChunks = Math.ceil(uploadFile.size / chunkSize)
    const sessionStart = await api.startUpload({
      document_id: crypto.randomUUID(),
      file_name: uploadFile.name,
      expected_chunks: totalChunks,
      expected_checksum: precomputedChecksum,
      duplicate_action: duplicateAction,
      metadata,
    })
    const sessionID = sessionStart.id ?? sessionStart.ID
    if (!sessionID) throw new Error('Invalid upload session')

    for (let index = 0; index < totalChunks; index++) {
      const start = index * chunkSize
      const end = Math.min(uploadFile.size, start + chunkSize)
      const raw = new Uint8Array(await uploadFile.slice(start, end).arrayBuffer())
      const checksum = await sha256Hex(raw)
      await api.uploadChunk({ session_id: sessionID, index, chunk_b64: uint8ToBase64(raw), checksum })
      setUploadProgress(Math.round(((index + 1) / totalChunks) * 100))
    }
    const version = await api.finalizeUpload(sessionID)
    setVersions((current) => [version, ...current])
    setUploadStatus('Upload completed')
    setItems((current) => [
      {
        ID: version.DocumentID,
        Title: version.FileName,
        CategoryID: metadata.category,
        Difficulty: metadata.difficulty,
        DurationMinutes: metadata.durationMinutes,
        Checksum: version.Checksum,
      },
      ...current,
    ])
  }

  async function resolveDuplicate() {
    setDuplicateModalOpen(false)
    if (pendingDuplicateChecksum) {
      try {
        await continueUpload(pendingDuplicateChecksum)
        setUploadStatus(`Duplicate resolved via "${duplicateAction}" and uploaded.`)
      } catch (error) {
        setUploadStatus(error instanceof Error ? error.message : 'Duplicate upload failed')
      } finally {
        setPendingDuplicateChecksum(null)
      }
      return
    }
    setUploadStatus(`Duplicate detected. Selected action: ${duplicateAction}`)
  }

  function generateShareLink() {
    if (!selectedItem) return
    api
      .generateShareLink(selectedItem.ID, metadata.shareExpiryHours)
      .then((result) => setShareLink(result.url))
      .catch((error) => setUploadStatus(error instanceof Error ? error.message : 'Unable to create share link'))
  }

  return {
    query,
    setQuery,
    categoryFilter,
    setCategoryFilter,
    tagFilter,
    setTagFilter,
    sortBy,
    setSortBy,
    items: filteredItems,
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
    setDuplicateModalOpen,
    duplicateAction,
    setDuplicateAction,
    shareLink,
    uploadInChunks,
    continueUpload,
    resolveDuplicate,
    generateShareLink,
  }
}

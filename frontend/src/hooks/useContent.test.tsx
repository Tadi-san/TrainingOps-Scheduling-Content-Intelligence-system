import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useContent } from './useContent'

function ContentHarness() {
  const { setUploadFile, uploadProgress, uploadStatus, uploadInChunks } = useContent()
  return (
    <div>
      <input
        aria-label="file"
        type="file"
        onChange={(event) => {
          const file = event.currentTarget.files?.[0] ?? null
          setUploadFile(file)
        }}
      />
      <button onClick={() => void uploadInChunks()}>upload</button>
      <div data-testid="progress">{uploadProgress}</div>
      <div data-testid="status">{uploadStatus}</div>
    </div>
  )
}

describe('useContent upload flow', () => {
  it('uploads chunks and marks success', async () => {
    const fetchSpy = vi.fn().mockImplementation(async (url: string) => {
      if (url.includes('/v1/uploads/start')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ id: 'upload-1' }),
        }
      }
      if (url.includes('/v1/uploads/finalize')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({
            ID: 'v-1',
            DocumentID: 'd-1',
            Version: 1,
            FileName: 'handbook.pdf',
            Checksum: 'abc',
            SizeBytes: 10,
            CreatedAt: new Date().toISOString(),
          }),
        }
      }
      return {
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ complete: true }),
      }
    })
    vi.stubGlobal('fetch', fetchSpy)

    render(<ContentHarness />)
    const file = new File(['hello world'], 'handbook.pdf', { type: 'application/pdf' })
    fireEvent.change(screen.getByLabelText('file'), { target: { files: [file] } })
    fireEvent.click(screen.getByText('upload'))

    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('Upload completed'))
    expect(Number(screen.getByTestId('progress').textContent ?? '0')).toBeGreaterThan(0)
  })
})

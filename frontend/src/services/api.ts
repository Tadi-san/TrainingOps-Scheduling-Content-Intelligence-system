export type Role = 'admin' | 'coordinator' | 'instructor' | 'learner'

export type AuthUser = {
  user_id: string
  tenant_id: string
  email: string
  display_name?: string
  role: Role
  status: string
  expires_at?: string
  session_id?: string
}

export type BookingStatus = 'held' | 'confirmed' | 'cancelled' | 'expired' | 'checked_in'
export type Room = {
  ID: string
  TenantID?: string
  Name: string
  Capacity: number
  CreatedAt?: string
  UpdatedAt?: string
}
export type Instructor = { ID: string; Name: string }

export type Booking = {
  ID: string
  TenantID: string
  UserID: string
  RoomID: string
  InstructorID: string
  Title: string
  StartAt: string
  EndAt: string
  Capacity: number
  Attendees: number
  Status: BookingStatus
  HoldExpiresAt?: string
  RescheduleCount: number
}

export type BookingConflict = {
  Reason: 'room' | 'instructor' | 'capacity'
  Detail: string
}

export type AlternativeSlot = {
  RoomID: string
  InstructorID: string
  StartAt: string
  EndAt: string
  Reason: string
}

export type KPI = { label: string; value: string; delta: string }
export type HeatmapCell = { day: string; hour: number; load: number; state: 'low' | 'medium' | 'high' }
export type Session = { id: string; title: string; startsAt: string; endsAt: string; room: string; owner: string; status: string }
export type DashboardData = {
  role: Role
  title: string
  subtitle: string
  kpis: KPI[]
  heatmap: HeatmapCell[]
  calendar: Session[]
  countdownEnd?: string
  taskOrdering: string[]
  previewDocument: string
  previewImage: string
}

export type ContentItem = {
  ID: string
  Title: string
  CategoryID: string
  Difficulty: number
  DurationMinutes: number
  Checksum?: string
  CreatedByUserID?: string
  UpdatedAt?: string
}

export type DocumentVersion = {
  ID: string
  DocumentID: string
  Version: number
  FileName: string
  Checksum: string
  SizeBytes: number
  CreatedAt: string
}

export type ShareResponse = {
  url: string
  token: string
  expires_at: string
}

export type Task = {
  ID: string
  MilestoneID: string
  Title: string
  Description: string
  DueDate?: string
  DependencyIDs: string[]
  EstimatedMinutes: number
  ActualMinutes: number
  Version: number
}

export type ClassPeriod = {
  id: string
  tenant_id: string
  title: string
  start_time: string
  end_time: string
  weekday: number
  version: number
  created_at: string
  updated_at: string
}

export type BlackoutDate = {
  id: string
  tenant_id: string
  date: string
  reason: string
  created_at: string
}

export type AdminTenant = {
  id: string
  name: string
  slug: string
  policies: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type AdminUser = {
  id: string
  tenant_id: string
  email: string
  display_name: string
  role: string
  created_at: string
}

export type Permission = {
  id: string
  key: string
}

export type SetupStatus = {
  needs_setup: boolean
}

export type SetupBootstrapResponse = {
  status: string
  tenant_id: string
  tenant_name: string
  tenant_slug: string
  admin_email: string
  admin_role: string
  next_action: string
  needs_setup: boolean
  setup_at_utc: string
}

export type LearnerCatalogItem = Booking

export type LearnerReservation = {
  ID: string
  TenantID: string
  BookingID: string
  LearnerUserID: string
  Status: string
  CreatedAt: string
  UpdatedAt: string
}

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? 'http://localhost:8080'

type APIError = {
  error?: string
  message?: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers ?? {})
  if (!headers.has('Content-Type') && init?.body) {
    headers.set('Content-Type', 'application/json')
  }
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  })

  const contentType = response.headers.get('content-type') ?? ''
  const json = contentType.includes('application/json') ? await response.json() : null
  if (!response.ok) {
    const err = (json ?? {}) as APIError
    throw new Error(err.error ?? err.message ?? `Request failed (${response.status})`)
  }
  return json as T
}

export const api = {
  async setupStatus() {
    return request<SetupStatus>('/v1/setup/status')
  },
  async bootstrapTenant(payload: {
    tenant_name: string
    tenant_slug: string
    admin_username: string
    admin_email: string
    admin_password: string
  }) {
    return request<SetupBootstrapResponse>('/v1/setup/tenant', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  async login(email: string, password: string, tenantID: string) {
    return request<AuthUser>('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password, tenant_id: tenantID }),
    })
  },
  async register(email: string, password: string, displayName: string, tenantID: string) {
    return request<{ status: string }>('/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, display_name: displayName, role: 'learner', tenant_id: tenantID }),
    })
  },
  async logout(sessionID?: string) {
    return request<{ status: string }>('/v1/auth/logout', {
      method: 'POST',
      headers: sessionID ? { 'X-Session-ID': sessionID } : undefined,
    })
  },
  async session() {
    return request<AuthUser>('/v1/auth/session')
  },
  async getDashboard(role: Role) {
    return request<DashboardData>(`/v1/workspaces/${role}/dashboard`)
  },
  async holdBooking(payload: Record<string, unknown>) {
    return request<{ booking?: Booking; conflicts?: BookingConflict[]; alternatives?: AlternativeSlot[]; error?: string }>('/v1/bookings/hold', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  async listRooms() {
    return request<{ items: Room[] }>('/v1/bookings/rooms')
  },
  async listInstructors() {
    return request<{ items: Instructor[] }>('/v1/bookings/instructors')
  },
  async listLearnerCatalog(params?: { room_id?: string; instructor_id?: string; from?: string; to?: string }) {
    const searchParams = new URLSearchParams()
    if (params?.room_id) searchParams.set('room_id', params.room_id)
    if (params?.instructor_id) searchParams.set('instructor_id', params.instructor_id)
    if (params?.from) searchParams.set('from', params.from)
    if (params?.to) searchParams.set('to', params.to)
    const suffix = searchParams.toString() ? `?${searchParams.toString()}` : ''
    return request<{ items: LearnerCatalogItem[] }>(`/v1/learner/catalog${suffix}`)
  },
  async reserveLearnerSeat(bookingID: string, why: string) {
    return request<{ reservation: LearnerReservation }>('/v1/learner/reserve', {
      method: 'POST',
      body: JSON.stringify({ booking_id: bookingID, why }),
    })
  },
  async listLearnerReservations() {
    return request<{ items: LearnerReservation[] }>('/v1/learner/my-reservations')
  },
  async downloadLearnerFile(fileID: string) {
    return `${API_BASE_URL}/v1/learner/download/${encodeURIComponent(fileID)}`
  },
  async listAdminRooms() {
    return request<{ items: Room[] }>('/v1/admin/rooms')
  },
  async createAdminRoom(payload: { name: string; capacity: number }) {
    return request<{ room: Room }>('/v1/admin/rooms', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  async updateAdminRoom(roomID: string, payload: { name: string; capacity: number }) {
    return request<{ room: Room }>(`/v1/admin/rooms/${encodeURIComponent(roomID)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
  },
  async listAdminTenants() {
    return request<{ items: AdminTenant[] }>('/v1/admin/tenants')
  },
  async updateTenantPolicies(tenantID: string, policies: Record<string, unknown>) {
    return request<{ status: string; tenant_id: string }>(`/v1/admin/tenants/${encodeURIComponent(tenantID)}/policies`, {
      method: 'PUT',
      body: JSON.stringify({ policies }),
    })
  },
  async listAdminUsers() {
    return request<{ items: AdminUser[] }>('/v1/admin/users')
  },
  async updateAdminUserRole(userID: string, role: 'admin' | 'coordinator' | 'instructor' | 'learner') {
    return request<{ status: string }>(`/v1/admin/users/${encodeURIComponent(userID)}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    })
  },
  async listPermissions() {
    return request<{ items: Permission[] }>('/v1/admin/permissions')
  },
  async confirmBooking(bookingID: string, why: string) {
    return request<{ status: string }>('/v1/bookings/confirm', { method: 'POST', body: JSON.stringify({ booking_id: bookingID, why }) })
  },
  async cancelBooking(bookingID: string, why: string) {
    return request<{ status: string }>('/v1/bookings/cancel', { method: 'POST', body: JSON.stringify({ booking_id: bookingID, why }) })
  },
  async rescheduleBooking(bookingID: string, newStart: string, newEnd: string, why: string) {
    const payload = JSON.stringify({ booking_id: bookingID, new_start: newStart, new_end: newEnd, why })
    try {
      return await request<{ booking: Booking }>('/v1/bookings/reschedule', {
        method: 'PUT',
        body: payload,
      })
    } catch (error) {
      const message = error instanceof Error ? error.message.toLowerCase() : ''
      if (!message.includes('404') && !message.includes('405')) throw error
      return request<{ booking: Booking }>('/v1/bookings/reschedule', {
        method: 'POST',
        body: payload,
      })
    }
  },
  async checkInBooking(bookingID: string) {
    return request<{ status: string }>(`/v1/bookings/${bookingID}/check-in`, { method: 'POST' })
  },
  async startUpload(payload: Record<string, unknown>) {
    return request<{ ID?: string; id?: string }>('/v1/uploads/start', { method: 'POST', body: JSON.stringify(payload) })
  },
  async uploadChunk(payload: Record<string, unknown>) {
    return request<{ complete: boolean }>('/v1/uploads/chunk', { method: 'POST', body: JSON.stringify(payload) })
  },
  async finalizeUpload(sessionID: string) {
    return request<DocumentVersion>('/v1/uploads/finalize', { method: 'POST', body: JSON.stringify({ session_id: sessionID }) })
  },
  async listDocumentVersions(documentID: string) {
    return request<{ document_id: string; versions: DocumentVersion[] }>(`/v1/content/${encodeURIComponent(documentID)}/versions`)
  },
  async generateShareLink(documentID: string, expiryHours: number) {
    return request<ShareResponse>(`/v1/content/${encodeURIComponent(documentID)}/share`, {
      method: 'POST',
      body: JSON.stringify({ expiry_hours: expiryHours }),
    })
  },
  async searchContent(q: string) {
    return request<{ query: string; items: ContentItem[] }>(`/v1/content/search?q=${encodeURIComponent(q)}`)
  },
  async listTasks(milestoneID: string) {
    return request<{ tasks: Task[] }>(`/v1/milestones/${milestoneID}/tasks`)
  },
  async listClassPeriods() {
    return request<{ items: ClassPeriod[] }>('/v1/schedule/class-periods')
  },
  async createClassPeriod(payload: { title: string; start_time: string; end_time: string; weekday: number }) {
    return request<ClassPeriod>('/v1/schedule/class-periods', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  async updateClassPeriod(periodID: string, payload: { title: string; start_time: string; end_time: string; weekday: number }) {
    return request<ClassPeriod>(`/v1/schedule/class-periods/${encodeURIComponent(periodID)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
  },
  async deleteClassPeriod(periodID: string) {
    return request<{ status: string }>(`/v1/schedule/class-periods/${encodeURIComponent(periodID)}`, { method: 'DELETE' })
  },
  async listBlackoutDates() {
    return request<{ items: BlackoutDate[] }>('/v1/schedule/blackout-dates')
  },
  async createBlackoutDate(payload: { date: string; reason: string }) {
    return request<BlackoutDate>('/v1/schedule/blackout-dates', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  async deleteBlackoutDate(id: string) {
    return request<{ status: string }>(`/v1/schedule/blackout-dates/${encodeURIComponent(id)}`, { method: 'DELETE' })
  },
  async updateTask(taskID: string, payload: Record<string, unknown>) {
    return request<Task>(`/v1/tasks/${taskID}`, { method: 'PUT', body: JSON.stringify(payload) })
  },
  async addTaskDependencies(taskID: string, dependencyIDs: string[]) {
    return request<Task>(`/v1/tasks/${taskID}/dependencies`, { method: 'POST', body: JSON.stringify({ dependency_ids: dependencyIDs }) })
  },
  async generateReport(type: 'seat' | 'near-expiry' | 'incident' | 'custom', format: 'csv' | 'pdf', dateFrom: string, dateTo: string) {
    const params = new URLSearchParams({
      type,
      format,
      date_from: dateFrom,
      date_to: dateTo,
    })
    const path = format === 'csv' ? `/v1/reports/bookings.csv?${params.toString()}` : `/v1/reports/compliance.pdf?${params.toString()}`
    return request<{ filename: string; size: number; created_at: string }>(path)
  },
  reportDownloadURL(filename: string) {
    return `${API_BASE_URL}/v1/reports/download/${encodeURIComponent(filename)}`
  },
}

export function formatTime(value: string) {
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export async function sha256Hex(data: Uint8Array) {
  const digest = await crypto.subtle.digest('SHA-256', data)
  const bytes = new Uint8Array(digest)
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

export function uint8ToBase64(bytes: Uint8Array) {
  let binary = ''
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

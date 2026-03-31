import { useEffect, useMemo, useState } from 'react'
import { AdminTenant, AdminUser, api, Permission, Room } from '../services/api'

type RoleOption = 'admin' | 'coordinator' | 'instructor' | 'learner'

export function AdminPage() {
  const [rooms, setRooms] = useState<Room[]>([])
  const [tenants, setTenants] = useState<AdminTenant[]>([])
  const [users, setUsers] = useState<AdminUser[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')

  const [createName, setCreateName] = useState('')
  const [createCapacity, setCreateCapacity] = useState(20)
  const [editingID, setEditingID] = useState('')
  const [editName, setEditName] = useState('')
  const [editCapacity, setEditCapacity] = useState(20)

  const [selectedTenantID, setSelectedTenantID] = useState('')
  const [policyDraft, setPolicyDraft] = useState('{}')

  async function loadAdminData() {
    setLoading(true)
    try {
      const [roomsPayload, tenantsPayload, usersPayload, permissionsPayload] = await Promise.all([
        api.listAdminRooms(),
        api.listAdminTenants(),
        api.listAdminUsers(),
        api.listPermissions(),
      ])
      const nextRooms = Array.isArray(roomsPayload.items) ? roomsPayload.items : []
      const nextTenants = Array.isArray(tenantsPayload.items) ? tenantsPayload.items : []
      const nextUsers = Array.isArray(usersPayload.items) ? usersPayload.items : []
      const nextPermissions = Array.isArray(permissionsPayload.items) ? permissionsPayload.items : []
      setRooms(nextRooms)
      setTenants(nextTenants)
      setUsers(nextUsers)
      setPermissions(nextPermissions)

      const defaultTenantID = selectedTenantID || nextTenants[0]?.id || ''
      setSelectedTenantID(defaultTenantID)
      const defaultTenant = nextTenants.find((item) => item.id === defaultTenantID)
      if (defaultTenant) {
        setPolicyDraft(JSON.stringify(defaultTenant.policies ?? {}, null, 2))
      }
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to load admin data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadAdminData()
  }, [])

  const selectedTenant = useMemo(() => tenants.find((tenant) => tenant.id === selectedTenantID), [tenants, selectedTenantID])

  async function createRoom() {
    if (!createName.trim() || createCapacity < 1) {
      setMessage('Room name and capacity are required')
      return
    }
    setLoading(true)
    try {
      await api.createAdminRoom({ name: createName.trim(), capacity: createCapacity })
      setCreateName('')
      setCreateCapacity(20)
      setMessage('Room created')
      await loadAdminData()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to create room')
      setLoading(false)
    }
  }

  function startEdit(room: Room) {
    setEditingID(room.ID)
    setEditName(room.Name)
    setEditCapacity(room.Capacity)
    setMessage('')
  }

  async function saveEdit() {
    if (!editingID) return
    if (!editName.trim() || editCapacity < 1) {
      setMessage('Room name and capacity are required')
      return
    }
    setLoading(true)
    try {
      await api.updateAdminRoom(editingID, { name: editName.trim(), capacity: editCapacity })
      setEditingID('')
      setEditName('')
      setEditCapacity(20)
      setMessage('Room updated')
      await loadAdminData()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update room')
      setLoading(false)
    }
  }

  async function savePolicies() {
    if (!selectedTenantID) {
      setMessage('Select a tenant first')
      return
    }
    let policies: Record<string, unknown>
    try {
      const parsed = JSON.parse(policyDraft)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        setMessage('Policies must be a JSON object')
        return
      }
      policies = parsed as Record<string, unknown>
    } catch {
      setMessage('Invalid policies JSON')
      return
    }
    setLoading(true)
    try {
      await api.updateTenantPolicies(selectedTenantID, policies)
      setMessage('Tenant policies updated')
      await loadAdminData()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update tenant policies')
      setLoading(false)
    }
  }

  async function updateUserRole(userID: string, role: RoleOption) {
    setLoading(true)
    try {
      await api.updateAdminUserRole(userID, role)
      setMessage('User role updated')
      await loadAdminData()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update user role')
      setLoading(false)
    }
  }

  return (
    <section className="content-grid">
      <article className="panel glass">
        <div className="panel-header">
          <h3>Create room</h3>
          <p>Add rooms used by scheduling and booking conflict checks.</p>
        </div>
        <label>
          Room name
          <input value={createName} onChange={(e) => setCreateName(e.target.value)} />
        </label>
        <label>
          Capacity
          <input type="number" min={1} value={createCapacity} onChange={(e) => setCreateCapacity(Number(e.target.value))} />
        </label>
        <button className="btn-primary" onClick={createRoom} disabled={loading}>
          Create room
        </button>
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Room inventory</h3>
          <p>Edit existing room capacity and names.</p>
        </div>
        <table className="data-grid">
          <thead>
            <tr>
              <th>Name</th>
              <th>Capacity</th>
              <th>Updated</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rooms.map((room) => (
              <tr key={room.ID}>
                <td>{room.Name}</td>
                <td>{room.Capacity}</td>
                <td>{room.UpdatedAt ? new Date(room.UpdatedAt).toLocaleString() : '-'}</td>
                <td>
                  <button className="btn-secondary" onClick={() => startEdit(room)} disabled={loading}>
                    Edit
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      {editingID ? (
        <article className="panel glass">
          <div className="panel-header">
            <h3>Edit room</h3>
            <p>Update room details and save.</p>
          </div>
          <label>
            Room name
            <input value={editName} onChange={(e) => setEditName(e.target.value)} />
          </label>
          <label>
            Capacity
            <input type="number" min={1} value={editCapacity} onChange={(e) => setEditCapacity(Number(e.target.value))} />
          </label>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn-primary" onClick={saveEdit} disabled={loading}>
              Save
            </button>
            <button className="btn-secondary" onClick={() => setEditingID('')} disabled={loading}>
              Cancel
            </button>
          </div>
        </article>
      ) : null}

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>Tenant policies</h3>
          <p>Administrator policy editing for tenant governance.</p>
        </div>
        <label>
          Tenant
          <select
            value={selectedTenantID}
            onChange={(e) => {
              const nextTenantID = e.target.value
              setSelectedTenantID(nextTenantID)
              const nextTenant = tenants.find((tenant) => tenant.id === nextTenantID)
              setPolicyDraft(JSON.stringify(nextTenant?.policies ?? {}, null, 2))
            }}
            disabled={loading}
          >
            {tenants.map((tenant) => (
              <option key={tenant.id} value={tenant.id}>
                {tenant.name} ({tenant.slug})
              </option>
            ))}
          </select>
        </label>
        <textarea value={policyDraft} onChange={(e) => setPolicyDraft(e.target.value)} rows={10} />
        <button className="btn-primary" onClick={savePolicies} disabled={loading || !selectedTenant}>
          Save policies
        </button>
      </article>

      <article className="panel glass panel-wide">
        <div className="panel-header">
          <h3>User roles</h3>
          <p>Manage role assignments across users.</p>
        </div>
        <table className="data-grid">
          <thead>
            <tr>
              <th>User</th>
              <th>Email</th>
              <th>Tenant</th>
              <th>Role</th>
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id}>
                <td>{user.display_name}</td>
                <td>{user.email}</td>
                <td>{user.tenant_id}</td>
                <td>
                  <select value={user.role} onChange={(e) => void updateUserRole(user.id, e.target.value as RoleOption)} disabled={loading}>
                    <option value="admin">admin</option>
                    <option value="coordinator">coordinator</option>
                    <option value="instructor">instructor</option>
                    <option value="learner">learner</option>
                  </select>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </article>

      <article className="panel glass">
        <div className="panel-header">
          <h3>Permissions</h3>
          <p>System permission catalog.</p>
        </div>
        {permissions.map((permission) => (
          <p key={permission.id} className="workspace-note">
            {permission.key}
          </p>
        ))}
      </article>

      {message ? (
        <article className="panel glass panel-wide">
          <p className="workspace-note">{message}</p>
        </article>
      ) : null}
    </section>
  )
}

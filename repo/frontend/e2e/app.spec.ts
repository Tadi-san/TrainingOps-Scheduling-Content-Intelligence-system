import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.route('**/v1/auth/login', async (route) => {
    const body = route.request().postDataJSON() as { email?: string }
    const role = body.email?.startsWith('admin') ? 'admin' : body.email?.startsWith('coordinator') ? 'coordinator' : 'learner'
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user_id: 'u-1',
        tenant_id: 'tenant-1',
        email: 'u***@example.com',
        role,
        status: 'authenticated',
      }),
    })
  })
  await page.route('**/v1/workspaces/**/dashboard', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        role: 'learner',
        title: 'Learner Dashboard',
        subtitle: 'Welcome',
        kpis: [{ label: 'Enrollment growth', value: '0%', delta: '0' }],
        heatmap: [],
        calendar: [],
        taskOrdering: [],
        previewDocument: '',
        previewImage: '',
      }),
    })
  })
  await page.route('**/v1/bookings/hold', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        booking: {
          ID: 'b-1',
          TenantID: 'tenant-1',
          UserID: 'u-1',
          RoomID: 'room-a',
          InstructorID: 'inst-1',
          Title: 'Class',
          StartAt: new Date(Date.now() + 48 * 60 * 60 * 1000).toISOString(),
          EndAt: new Date(Date.now() + 49 * 60 * 60 * 1000).toISOString(),
          Capacity: 20,
          Attendees: 2,
          Status: 'held',
          HoldExpiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
          RescheduleCount: 0,
        },
      }),
    })
  })
  await page.route('**/v1/bookings/confirm', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'confirmed' }) })
  })
  await page.route('**/v1/content/search**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ query: '', items: [] }) })
  })
  await page.route('**/v1/uploads/start', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: 'upload-1' }) })
  })
  await page.route('**/v1/uploads/chunk', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ complete: true }) })
  })
  await page.route('**/v1/uploads/finalize', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ID: 'v-1',
        DocumentID: 'd-1',
        Version: 1,
        FileName: 'course.pdf',
        Checksum: 'abc',
        SizeBytes: 10,
        CreatedAt: new Date().toISOString(),
      }),
    })
  })
  await page.route('**/v1/auth/logout', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'revoked' }) })
  })
})

test('login -> dashboard', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@example.com')
  await page.getByLabel('Password').fill('Password123!')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page.getByText('Workspace')).toBeVisible()
})

test('learner booking hold -> confirm', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('learner@example.com')
  await page.getByLabel('Password').fill('Password123!')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await page.getByText('Bookings').click()
  await page.getByRole('button', { name: 'Start Hold' }).click()
  await expect(page.getByText('Booking confirmed.')).not.toBeVisible()
  await page.getByRole('button', { name: 'Confirm' }).click()
  await expect(page.getByText('Booking confirmed.')).toBeVisible()
})

test('logout blocks protected page', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('coordinator@example.com')
  await page.getByLabel('Password').fill('Password123!')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await page.getByRole('button', { name: 'Log out' }).click()
  await expect(page).toHaveURL(/\/login/)
})

test('coordinator creates content upload and sees completion', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('coordinator@example.com')
  await page.getByLabel('Password').fill('Password123!')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await page.getByText('Content').click()

  const chooser = page.waitForEvent('filechooser')
  await page.locator('input[type="file"]').click()
  const fileChooser = await chooser
  await fileChooser.setFiles({
    name: 'course.pdf',
    mimeType: 'application/pdf',
    buffer: Buffer.from('pdf-content'),
  })
  await page.getByRole('button', { name: 'Upload in chunks' }).click()
  await expect(page.getByText('Upload completed')).toBeVisible()
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DataTable from '../DataTable.vue'

describe('DataTable', () => {
  const columns = [
    { key: 'name', label: 'Name', sortable: true },
    { key: 'status', label: 'Status' },
  ]

  const rows = [
    { name: 'Source A', status: 'active' },
    { name: 'Source B', status: 'paused' },
  ]

  it('renders column headers', () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    const headers = wrapper.findAll('th')
    expect(headers).toHaveLength(2)
    expect(headers[0].text()).toBe('Name')
    expect(headers[1].text()).toBe('Status')
  })

  it('renders rows', () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    const cells = wrapper.findAll('td')
    expect(cells).toHaveLength(4)
    expect(cells[0].text()).toBe('Source A')
  })

  it('shows loading skeleton when loading', () => {
    const wrapper = mount(DataTable, { props: { columns, rows: [], loading: true } })
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('shows empty message when no rows', () => {
    const wrapper = mount(DataTable, { props: { columns, rows: [] } })
    expect(wrapper.text()).toContain('No data available')
  })

  it('emits sort event on sortable column click', async () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    await wrapper.findAll('th')[0].trigger('click')
    expect(wrapper.emitted('sort')).toEqual([['name']])
  })
})

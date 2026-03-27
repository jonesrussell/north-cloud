import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusBadge from '../StatusBadge.vue'

describe('StatusBadge', () => {
  it('renders status text', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'active' } })
    expect(wrapper.text()).toBe('active')
  })

  it('applies green classes for active status', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'active' } })
    expect(wrapper.find('span').classes()).toContain('text-green-400')
  })

  it('applies red classes for error status', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'error' } })
    expect(wrapper.find('span').classes()).toContain('text-red-400')
  })

  it('applies default classes for unknown status', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'unknown' } })
    expect(wrapper.find('span').classes()).toContain('text-slate-400')
  })
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoadingSkeleton from '../LoadingSkeleton.vue'

describe('LoadingSkeleton', () => {
  it('renders default 3 lines', () => {
    const wrapper = mount(LoadingSkeleton)
    expect(wrapper.findAll('.animate-pulse')).toHaveLength(3)
  })

  it('renders specified number of lines', () => {
    const wrapper = mount(LoadingSkeleton, { props: { lines: 5 } })
    expect(wrapper.findAll('.animate-pulse')).toHaveLength(5)
  })
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ErrorBanner from '../ErrorBanner.vue'

describe('ErrorBanner', () => {
  it('renders error message', () => {
    const wrapper = mount(ErrorBanner, { props: { message: 'Something went wrong' } })
    expect(wrapper.text()).toContain('Something went wrong')
  })

  it('emits retry on button click', async () => {
    const wrapper = mount(ErrorBanner, { props: { message: 'Error' } })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })
})

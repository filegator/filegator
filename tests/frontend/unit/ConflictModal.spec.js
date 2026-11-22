import { shallowMount } from '@vue/test-utils'
import ConflictModal from '@/views/partials/ConflictModal.vue'

describe('ConflictModal.vue', () => {
  const conflicts = [
    { name: 'setup.php', type: 'file' },
    { name: 'inc', type: 'dir' },
  ]

  const mockParent = {
    close: jest.fn(),
  }

  const mocks = {
    $parent: mockParent,
    lang: key => key,
  }

  it('renders conflicting items', () => {
    const wrapper = shallowMount(ConflictModal, {
      propsData: { conflicts },
      mocks,
      stubs: ['b-icon'],
    })

    expect(wrapper.text()).toContain('setup.php')
    expect(wrapper.text()).toContain('inc')
    expect(wrapper.findAll('.conflict-item').length).toBe(2)
  })

  it('emits resolve with replace when replace button clicked', async () => {
    const wrapper = shallowMount(ConflictModal, {
      propsData: { conflicts },
      mocks,
      stubs: ['b-icon'],
    })

    const buttons = wrapper.findAll('button')
    const replaceBtn = buttons.filter(b => b.text().includes('Replace')).at(0)
    await replaceBtn.trigger('click')

    expect(wrapper.emitted().resolve).toBeTruthy()
    expect(wrapper.emitted().resolve[0]).toEqual(['replace'])
    expect(mockParent.close).toHaveBeenCalled()
  })

  it('emits resolve with rename when rename button clicked', async () => {
    const wrapper = shallowMount(ConflictModal, {
      propsData: { conflicts },
      mocks,
      stubs: ['b-icon'],
    })

    const buttons = wrapper.findAll('button')
    const renameBtn = buttons.filter(b => b.text().includes('Rename')).at(0)
    await renameBtn.trigger('click')

    expect(wrapper.emitted().resolve).toBeTruthy()
    expect(wrapper.emitted().resolve[0]).toEqual(['rename'])
  })
})

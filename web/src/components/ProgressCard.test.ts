import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import ProgressCard from './ProgressCard.vue';

describe('ProgressCard', () => {
  it('renders label and value', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Sessions', value: '42' },
    });

    expect(wrapper.text()).toContain('Sessions');
    expect(wrapper.text()).toContain('42');
  });

  it('renders numeric value', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Score', value: 8.5 },
    });

    expect(wrapper.text()).toContain('8.5');
  });

  it('renders subtitle when provided', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Time', value: '30m', subtitle: 'this week' },
    });

    expect(wrapper.text()).toContain('this week');
  });

  it('renders positive trend', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Skill', value: '7.2', trend: 5.3 },
    });

    expect(wrapper.text()).toContain('+5.3%');
  });

  it('renders negative trend', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Errors', value: '3', trend: -2.1 },
    });

    expect(wrapper.text()).toContain('-2.1%');
  });

  it('does not render trend when not provided', () => {
    const wrapper = mount(ProgressCard, {
      props: { label: 'Count', value: '10' },
    });

    expect(wrapper.find('.card-trend').exists()).toBe(false);
  });
});

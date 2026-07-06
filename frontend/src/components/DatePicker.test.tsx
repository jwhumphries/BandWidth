import {format} from 'date-fns';
import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import DatePicker from './DatePicker';

describe('DatePicker', () => {
  it('shows the placeholder when empty and opens the calendar on click', async () => {
    render(
      <DatePicker value="" onChange={vi.fn()} aria-label="Backfill date" />,
    );
    expect(screen.getByText('Select date')).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('Backfill date'));
    expect(screen.getByRole('button', {name: /^today,/i})).toBeInTheDocument();
  });

  it('calls onChange with the selected date formatted as yyyy-MM-dd', async () => {
    const onChange = vi.fn();
    render(
      <DatePicker value="" onChange={onChange} aria-label="Backfill date" />,
    );

    await userEvent.click(screen.getByLabelText('Backfill date'));
    await userEvent.click(screen.getByRole('button', {name: /^today,/i}));

    expect(onChange).toHaveBeenCalledWith(format(new Date(), 'yyyy-MM-dd'));
  });

  it('displays a formatted value when one is set', () => {
    render(
      <DatePicker
        value="2026-05-01"
        onChange={vi.fn()}
        aria-label="Backfill date"
      />,
    );
    expect(screen.getByText('May 1, 2026')).toBeInTheDocument();
  });
});

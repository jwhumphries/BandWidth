import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import type {SongListItem} from '../../lib/types';
import PracticeRow from './PracticeRow';

const song: SongListItem = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  lastPracticedAt: '2026-01-01',
  practiceCount: 2,
};

describe('PracticeRow', () => {
  it('shows the action label and fires onToggle when not done', async () => {
    const onToggle = vi.fn();
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done={false}
          actionLabel="Practiced"
          canAct
          onToggle={onToggle}
        />
      </ul>,
    );
    const button = screen.getByRole('button', {name: /practiced/i});
    await userEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('shows an Undo button when done', () => {
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done
          actionLabel="Practiced"
          canAct
          onToggle={vi.fn()}
        />
      </ul>,
    );
    expect(screen.getByRole('button', {name: /undo/i})).toBeInTheDocument();
  });

  it('disables the button when the user cannot act', () => {
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done={false}
          actionLabel="Rehearsed"
          canAct={false}
          onToggle={vi.fn()}
        />
      </ul>,
    );
    expect(screen.getByRole('button', {name: /rehearsed/i})).toBeDisabled();
  });
});

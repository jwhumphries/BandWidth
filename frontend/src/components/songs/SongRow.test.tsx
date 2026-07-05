import {render, screen} from '@testing-library/react';
import {MemoryRouter} from 'react-router';
import {describe, expect, it} from 'vitest';
import SongRow from './SongRow';

const song = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learned' as const,
  lastPracticedAt: '',
  practiceCount: 0,
};

function renderRow(canEdit: boolean) {
  return render(
    <MemoryRouter>
      <SongRow
        song={song}
        linkTo="/songs/1"
        onPracticed={() => {}}
        canEdit={canEdit}
      />
    </MemoryRouter>,
  );
}

describe('SongRow', () => {
  it('shows the action button when canEdit is true', () => {
    renderRow(true);
    expect(
      screen.getByRole('button', {name: /practiced/i}),
    ).toBeInTheDocument();
  });

  it('hides the action button when canEdit is false', () => {
    renderRow(false);
    expect(
      screen.queryByRole('button', {name: /practiced/i}),
    ).not.toBeInTheDocument();
  });
});

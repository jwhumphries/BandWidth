import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import PracticeSourcePicker from './PracticeSourcePicker';

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

const folders = [{id: 1, name: 'Warmups', position: 1, songIds: []}];
const bands = [
  {id: 5, name: 'The Cure', creatorId: 1, role: 'editor', memberCount: 2},
];
const bandFolders = [{id: 9, name: 'Setlist', position: 1, songIds: []}];

describe('PracticeSourcePicker', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/bands/5/folders'))
          return Promise.resolve(json(200, bandFolders));
        if (url.endsWith('/api/bands'))
          return Promise.resolve(json(200, bands));
        if (url.endsWith('/api/folders'))
          return Promise.resolve(json(200, folders));
        return Promise.resolve(json(200, []));
      }),
    );
  });

  it('renders every source group and emits encoded values', async () => {
    const onChange = vi.fn();
    renderWithProviders(
      <PracticeSourcePicker value="all" onChange={onChange} />,
    );

    expect(screen.getByRole('option', {name: 'All Songs'})).toBeInTheDocument();
    expect(
      await screen.findByRole('option', {name: 'Warmups'}),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole('option', {name: 'All The Cure songs'}),
    ).toBeInTheDocument();

    // Band folder options load lazily once the select is opened/focused.
    await userEvent.click(screen.getByRole('combobox'));
    expect(
      await screen.findByRole('option', {name: 'Setlist'}),
    ).toBeInTheDocument();

    await userEvent.selectOptions(
      screen.getByRole('combobox'),
      'bandfolder:5:9',
    );
    expect(onChange).toHaveBeenCalledWith('bandfolder:5:9');
  });
});

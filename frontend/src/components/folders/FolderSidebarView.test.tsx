import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import FolderSidebarView from './FolderSidebarView';
import type {Folder} from '../../lib/types';

const folders: Folder[] = [
  {id: 1, name: 'Set 1', position: 1, songIds: []},
  {id: 2, name: 'Encore', position: 2, songIds: []},
];

function renderView(props: Partial<Parameters<typeof FolderSidebarView>[0]>) {
  const onSelect = vi.fn();
  const onCreate = vi.fn();
  render(
    <FolderSidebarView
      folders={folders}
      canEdit
      selectedId={null}
      onSelect={onSelect}
      onCreate={onCreate}
      onRename={vi.fn()}
      onDelete={vi.fn()}
      onReorder={vi.fn()}
      {...props}
    />,
  );
  return {onSelect, onCreate};
}

describe('FolderSidebarView', () => {
  it('selects all songs and individual folders', async () => {
    const {onSelect} = renderView({selectedId: 1});

    await userEvent.click(screen.getByRole('button', {name: /all songs/i}));
    expect(onSelect).toHaveBeenCalledWith(null);

    await userEvent.click(screen.getByRole('button', {name: 'Encore'}));
    expect(onSelect).toHaveBeenCalledWith(2);
  });

  it('creates a folder from the icon button and clears the input', async () => {
    const {onCreate} = renderView({});

    const input = screen.getByPlaceholderText(/new folder/i);
    await userEvent.type(input, '  Gigs  ');
    await userEvent.click(screen.getByRole('button', {name: /create/i}));

    expect(onCreate).toHaveBeenCalledWith('Gigs');
    expect(input).toHaveValue('');
  });

  it('ignores a blank folder name', async () => {
    const {onCreate} = renderView({});

    await userEvent.type(screen.getByPlaceholderText(/new folder/i), '   ');
    await userEvent.click(screen.getByRole('button', {name: /create/i}));

    expect(onCreate).not.toHaveBeenCalled();
  });

  it('hides every editing control for viewers', () => {
    renderView({canEdit: false});

    expect(screen.getByRole('button', {name: 'Set 1'})).toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText(/new folder/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /rename set 1/i}),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /delete set 1/i}),
    ).not.toBeInTheDocument();
  });

  it('confirms before deleting a folder', async () => {
    const onDelete = vi.fn();
    renderView({onDelete});

    await userEvent.click(screen.getByRole('button', {name: /delete set 1/i}));
    expect(onDelete).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole('button', {name: /^delete$/i}));
    expect(onDelete).toHaveBeenCalledWith(folders[0]);
  });
});

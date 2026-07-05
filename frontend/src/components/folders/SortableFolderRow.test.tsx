import {DndContext} from '@dnd-kit/core';
import {SortableContext} from '@dnd-kit/sortable';
import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import SortableFolderRow from './SortableFolderRow';

const folder = {id: 1, name: 'Set 1', position: 1, songIds: []};

function renderRow(canEdit: boolean) {
  return render(
    <DndContext>
      <SortableContext items={[folder.id]}>
        <ul>
          <SortableFolderRow
            folder={folder}
            canEdit={canEdit}
            selected={false}
            onSelect={() => {}}
            onRename={() => {}}
            onDelete={() => {}}
          />
        </ul>
      </SortableContext>
    </DndContext>,
  );
}

describe('SortableFolderRow', () => {
  it('shows drag, rename, and delete affordances when canEdit is true', () => {
    renderRow(true);
    expect(
      screen.getByRole('button', {name: /reorder set 1/i}),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', {name: /rename set 1/i}),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', {name: /delete set 1/i}),
    ).toBeInTheDocument();
  });

  it('hides drag, rename, and delete affordances when canEdit is false', () => {
    renderRow(false);
    expect(
      screen.queryByRole('button', {name: /reorder set 1/i}),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /rename set 1/i}),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /delete set 1/i}),
    ).not.toBeInTheDocument();
  });
});

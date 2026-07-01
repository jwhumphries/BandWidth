import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {SortableContext, verticalListSortingStrategy} from '@dnd-kit/sortable';
import {useState} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../songs/ConfirmModal';
import BandFolderRow from './BandFolderRow';
import {
  useBandFolders,
  useCreateBandFolder,
  useDeleteBandFolder,
  useRenameBandFolder,
  useReorderBandFolders,
} from '../../hooks/bandfolders';
import type {Folder} from '../../lib/types';

export default function BandFolderSidebar({
  bandId,
  canEdit,
  selectedId,
  onSelect,
}: {
  bandId: number;
  canEdit: boolean;
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useBandFolders(bandId);
  const createFolder = useCreateBandFolder(bandId);
  const renameFolder = useRenameBandFolder(bandId);
  const deleteFolder = useDeleteBandFolder(bandId);
  const reorderFolders = useReorderBandFolders(bandId);
  const [newName, setNewName] = useState('');
  const [deleting, setDeleting] = useState<Folder | null>(null);

  const create = (e: FormEvent) => {
    e.preventDefault();
    if (!newName.trim()) return;
    createFolder.mutate(
      {name: newName.trim()},
      {onSuccess: () => setNewName('')},
    );
  };

  const dragEnd = (event: DragEndEvent) => {
    const {active, over} = event;
    if (!over || active.id === over.id) return;
    const ids = folders.map(f => f.id);
    const from = ids.indexOf(Number(active.id));
    const to = ids.indexOf(Number(over.id));
    if (from === -1 || to === -1) return;
    ids.splice(to, 0, ...ids.splice(from, 1));
    reorderFolders.mutate(ids);
  };

  return (
    <aside className="w-full sm:w-56">
      <ul className="menu bg-base-100 rounded-box w-full p-2">
        <li>
          <button
            className={selectedId === null ? 'active' : ''}
            onClick={() => onSelect(null)}
          >
            All songs
          </button>
        </li>
      </ul>
      <DndContext collisionDetection={closestCenter} onDragEnd={dragEnd}>
        <SortableContext
          items={folders.map(f => f.id)}
          strategy={verticalListSortingStrategy}
        >
          <ul className="mt-2 flex flex-col gap-1">
            {folders.map(f => (
              <BandFolderRow
                key={f.id}
                folder={f}
                canEdit={canEdit}
                selected={selectedId === f.id}
                onSelect={() => onSelect(f.id)}
                onRename={name => renameFolder.mutate({id: f.id, name})}
                onDelete={() => setDeleting(f)}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      {canEdit && (
        <form className="mt-3 flex gap-1" onSubmit={create}>
          <input
            className="input input-sm min-w-0 flex-1"
            placeholder="New folder…"
            value={newName}
            onChange={e => setNewName(e.target.value)}
          />
          <button className="btn btn-sm" disabled={createFolder.isPending}>
            Create
          </button>
        </form>
      )}
      <ConfirmModal
        open={deleting !== null}
        title="Delete folder"
        message={`Delete "${deleting?.name ?? ''}"? Songs in it are not deleted.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleting) {
            deleteFolder.mutate(deleting.id, {
              onSuccess: () => {
                if (selectedId === deleting.id) onSelect(null);
              },
            });
          }
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </aside>
  );
}

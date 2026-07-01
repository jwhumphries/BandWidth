import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {SortableContext, verticalListSortingStrategy} from '@dnd-kit/sortable';
import {ListMusic, Plus} from 'lucide-react';
import {useState} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../songs/ConfirmModal';
import SortableFolderRow from './SortableFolderRow';
import {
  useCreateFolder,
  useDeleteFolder,
  useFolders,
  useRenameFolder,
  useReorderFolders,
} from '../../hooks/folders';
import type {Folder} from '../../lib/types';

export default function FolderSidebar({
  selectedId,
  onSelect,
}: {
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useFolders();
  const createFolder = useCreateFolder();
  const renameFolder = useRenameFolder();
  const deleteFolder = useDeleteFolder();
  const reorderFolders = useReorderFolders();
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
    <aside className="border-base-300/60 bg-base-100 h-fit w-full rounded-box border p-3 sm:w-60 sm:shrink-0">
      <button
        className={`flex w-full items-center gap-2 rounded-field px-2 py-1.5 text-sm font-semibold transition-colors ${
          selectedId === null
            ? 'bg-base-300 text-base-content'
            : 'text-base-content/75 hover:bg-base-300/50'
        }`}
        onClick={() => onSelect(null)}
      >
        <ListMusic className="size-4" />
        All songs
      </button>
      <p className="text-base-content/40 mt-4 mb-1 px-2 text-xs font-semibold tracking-wide uppercase">
        Folders
      </p>
      <DndContext collisionDetection={closestCenter} onDragEnd={dragEnd}>
        <SortableContext
          items={folders.map(f => f.id)}
          strategy={verticalListSortingStrategy}
        >
          <ul className="flex flex-col gap-0.5">
            {folders.map(f => (
              <SortableFolderRow
                key={f.id}
                folder={f}
                selected={selectedId === f.id}
                onSelect={() => onSelect(f.id)}
                onRename={name => renameFolder.mutate({id: f.id, name})}
                onDelete={() => setDeleting(f)}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      <form className="mt-3 flex gap-1" onSubmit={create}>
        <input
          className="input input-sm min-w-0 flex-1"
          placeholder="New folder…"
          value={newName}
          onChange={e => setNewName(e.target.value)}
        />
        <button
          className="btn btn-sm btn-square btn-primary"
          aria-label="Create"
          disabled={createFolder.isPending}
        >
          <Plus className="size-4" />
        </button>
      </form>
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

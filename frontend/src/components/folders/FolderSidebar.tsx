import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {useState, useRef} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../songs/ConfirmModal';
import {
  useCreateFolder,
  useDeleteFolder,
  useFolders,
  useRenameFolder,
  useReorderFolders,
} from '../../hooks/folders';
import type {Folder} from '../../lib/types';

function SortableFolderRow({
  folder,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  selected: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDelete: () => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: folder.id});
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(folder.name);
  const submitted = useRef(false);

  const submitRename = (e: FormEvent) => {
    e.preventDefault();
    if (submitted.current) {
      return;
    }
    submitted.current = true;
    if (name.trim()) {
      onRename(name.trim());
    }
    setEditing(false);
  };

  return (
    <li
      ref={setNodeRef}
      style={{transform: CSS.Transform.toString(transform), transition}}
      className={`flex items-center gap-1 rounded-box px-2 py-1 ${
        selected ? 'bg-base-300' : ''
      }`}
    >
      <button
        className="cursor-grab touch-none"
        aria-label={`Reorder ${folder.name}`}
        {...attributes}
        {...listeners}
      >
        ⠿
      </button>
      {editing ? (
        <form onSubmit={submitRename} className="flex-1">
          <input
            className="input input-xs w-full"
            value={name}
            onChange={e => setName(e.target.value)}
            autoFocus
            onBlur={submitRename}
          />
        </form>
      ) : (
        <button
          className="min-w-0 flex-1 truncate text-left"
          onClick={onSelect}
        >
          {folder.name}
        </button>
      )}
      <button
        className="btn btn-ghost btn-xs"
        aria-label={`Rename ${folder.name}`}
        onClick={() => {
          submitted.current = false;
          setName(folder.name);
          setEditing(true);
        }}
      >
        ✎
      </button>
      <button
        className="btn btn-ghost btn-xs"
        aria-label={`Delete ${folder.name}`}
        onClick={onDelete}
      >
        ✕
      </button>
    </li>
  );
}

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
        <button className="btn btn-sm" disabled={createFolder.isPending}>
          Create
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

import {useSortable} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {useState, useRef} from 'react';
import type {FormEvent} from 'react';
import type {Folder} from '../../lib/types';

export default function BandFolderRow({
  folder,
  canEdit,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  canEdit: boolean;
  selected: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDelete: () => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: folder.id, disabled: !canEdit});
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(folder.name);
  const submitted = useRef(false);

  const submitRename = (e: FormEvent) => {
    e.preventDefault();
    if (submitted.current) return;
    submitted.current = true;
    if (name.trim()) onRename(name.trim());
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
      {canEdit && (
        <button
          className="cursor-grab touch-none"
          aria-label={`Reorder ${folder.name}`}
          {...attributes}
          {...listeners}
        >
          ⠿
        </button>
      )}
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
      {canEdit && (
        <>
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
        </>
      )}
    </li>
  );
}

import {useSortable} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {GripVertical, Pencil, X} from 'lucide-react';
import {useState, useRef} from 'react';
import type {FormEvent} from 'react';
import type {Folder} from '../../lib/types';

export default function SortableFolderRow({
  folder,
  canEdit = true,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  canEdit?: boolean;
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
      className={`group flex items-center gap-1 rounded-field px-1.5 py-1 transition-colors ${
        selected ? 'bg-base-300' : 'hover:bg-base-300/50'
      }`}
    >
      {canEdit && (
        <button
          className="text-base-content/30 hover:text-base-content/70 cursor-grab touch-none px-0.5"
          aria-label={`Reorder ${folder.name}`}
          {...attributes}
          {...listeners}
        >
          <GripVertical className="size-4" />
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
          className={`min-w-0 flex-1 truncate text-left text-sm font-medium ${
            selected ? '' : 'text-base-content/75'
          }`}
          onClick={onSelect}
        >
          {folder.name}
        </button>
      )}
      {canEdit && (
        <>
          <button
            className="btn btn-ghost btn-xs btn-square opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
            aria-label={`Rename ${folder.name}`}
            onClick={() => {
              submitted.current = false;
              setName(folder.name);
              setEditing(true);
            }}
          >
            <Pencil className="size-3.5" />
          </button>
          <button
            className="btn btn-ghost btn-xs btn-square hover:text-error opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
            aria-label={`Delete ${folder.name}`}
            onClick={onDelete}
          >
            <X className="size-3.5" />
          </button>
        </>
      )}
    </li>
  );
}
